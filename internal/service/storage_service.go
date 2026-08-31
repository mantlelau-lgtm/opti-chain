package service

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"scm/internal/database"
	"scm/internal/model"
	"scm/internal/repository"
)

// tabler exposes a GORM model's table name.
type tabler interface{ TableName() string }

// MigrationProgress is the state surfaced to the platform console.
type MigrationProgress struct {
	Status       string `json:"status"` // idle | running | done | failed
	TargetName   string `json:"target_name"`
	CurrentTable string `json:"current_table"`
	TotalTables  int    `json:"total_tables"`
	DoneTables   int    `json:"done_tables"`
	TotalRows    int64  `json:"total_rows"`
	DoneRows     int64  `json:"done_rows"`
	Error        string `json:"error"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
}

// StorageService owns data-source configuration and cross-database migration.
type StorageService struct {
	db     *gorm.DB // the live (source) database
	repo   *repository.DataSourceRepo
	driver string // current active driver
	dsn    string // current active dsn
	mu     sync.Mutex
	state  MigrationProgress
}

func NewStorageService(db *gorm.DB, repo *repository.DataSourceRepo, driver, dsn string) *StorageService {
	return &StorageService{db: db, repo: repo, driver: driver, dsn: dsn, state: MigrationProgress{Status: "idle"}}
}

// Current returns the active data-source config (driver + dsn).
func (s *StorageService) Current() (string, string) {
	return s.driver, s.dsn
}

// ---- data source CRUD ----

func (s *StorageService) ListDataSources(in PageInput) ([]model.DataSource, int64, error) {
	var (
		out   []model.DataSource
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *StorageService) CreateDataSource(ds *model.DataSource) error {
	if ds.Name == "" || ds.DSN == "" {
		return errorsBadRequest("name and dsn are required")
	}
	if ds.Driver != "mysql" && ds.Driver != "postgres" {
		return errorsBadRequest("driver must be mysql or postgres")
	}
	if err := s.testConnection(ds.Driver, ds.DSN); err != nil {
		return errf(ErrBadRequest, "connection failed: "+err.Error())
	}
	return s.repo.Create(ds)
}

func (s *StorageService) DeleteDataSource(id uint) error {
	return s.repo.Delete(id)
}

// TestConnection opens + pings a data source without persisting anything.
func (s *StorageService) TestConnection(driver, dsn string) error {
	return s.testConnection(driver, dsn)
}

func (s *StorageService) testConnection(driver, dsn string) error {
	gdb, err := openTarget(driver, dsn)
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return sqlDB.Ping()
}

// Status returns the current migration progress (thread-safe snapshot).
func (s *StorageService) Status() MigrationProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Migrate starts a background migration from the live DB to the configured
// target data source. Only one migration runs at a time.
func (s *StorageService) Migrate(dsID uint) error {
	s.mu.Lock()
	if s.state.Status == "running" {
		s.mu.Unlock()
		return errorsBadRequest("a migration is already running")
	}
	s.state = MigrationProgress{Status: "running", StartedAt: time.Now().Format(time.RFC3339)}
	s.mu.Unlock()

	ds, err := s.repo.Get(dsID)
	if err != nil || ds == nil {
		s.fail("data source not found")
		return errNotFound(dsID)
	}

	go s.run(ds)
	return nil
}

func (s *StorageService) fail(msg string) {
	s.mu.Lock()
	s.state.Status = "failed"
	s.state.Error = msg
	s.state.FinishedAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()
}

func (s *StorageService) setRunning(name, table string, total, done int, rows int64) {
	s.mu.Lock()
	s.state.TargetName = name
	s.state.CurrentTable = table
	s.state.TotalTables = total
	s.state.DoneTables = done
	s.state.TotalRows = rows
	s.mu.Unlock()
}

// run performs the copy on a background goroutine.
func (s *StorageService) run(ds *model.DataSource) {
	s.setRunning(ds.Name, "", 0, 0, 0)

	gdb, err := openTarget(ds.Driver, ds.DSN)
	if err != nil {
		s.fail("open target: " + err.Error())
		return
	}
	sqlDB, _ := gdb.DB()
	if sqlDB != nil {
		defer sqlDB.Close()
	}

	models := database.Models()

	// Drop any existing app tables in the target first, so re-running a
	// migration into a non-empty target is idempotent (no PK collisions).
	for i := len(models) - 1; i >= 0; i-- {
		_ = gdb.Migrator().DropTable(models[i])
	}

	if err := database.Migrate(gdb); err != nil {
		s.fail("migrate target schema: " + err.Error())
		return
	}

	// total row count for the progress bar.
	var totalRows int64
	tableNames := make([]string, 0, len(models))
	for _, m := range models {
		tableNames = append(tableNames, m.(tabler).TableName())
	}
	for _, t := range tableNames {
		var n int64
		_ = s.db.Raw("SELECT COUNT(*) AS n FROM " + t).Scan(&n).Error
		totalRows += n
	}
	s.setRunning(ds.Name, "准备复制", len(models), 0, totalRows)

	// Copy inside ONE transaction so the FK-disabling statement and the INSERTs
	// share a single pooled connection (MySQL's FOREIGN_KEY_CHECKS is
	// session-scoped). This tolerates orphaned detail rows from the source.
	var doneRows int64
	err = gdb.Transaction(func(tx *gorm.DB) error {
		disableFK(tx, ds.Driver)
		for i, m := range models {
			name := m.(tabler).TableName()
			s.mu.Lock()
			s.state.CurrentTable = name
			s.mu.Unlock()

			n, err := s.copyTable(s.db, tx, m)
			if err != nil {
				return fmt.Errorf("copy table %s: %w", name, err)
			}
			doneRows += n
			s.mu.Lock()
			s.state.DoneTables = i + 1
			s.state.DoneRows = doneRows
			s.mu.Unlock()
		}
		return nil
	})
	if err != nil {
		s.fail(err.Error())
		return
	}

	// Postgres: sequences must advance past the copied max ids.
	if ds.Driver == "postgres" {
		for _, t := range tableNames {
			_ = gdb.Exec(fmt.Sprintf(
				"SELECT setval(pg_get_serial_sequence('%s','id'), COALESCE(MAX(id),1)) FROM %s", t, t)).Error
		}
	}

	s.mu.Lock()
	s.state.Status = "done"
	s.state.CurrentTable = ""
	s.state.FinishedAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()
}

// copyTable copies all rows of one table from src to dst using the typed model,
// so GORM serializes datetimes/decimals correctly and preserves ids + original
// timestamps (autoCreateTime only fills zero values).
func (s *StorageService) copyTable(src, dst *gorm.DB, model interface{}) (int64, error) {
	elemType := reflect.TypeOf(model).Elem()
	slicePtr := reflect.New(reflect.SliceOf(elemType)).Interface()
	if err := src.Find(slicePtr).Error; err != nil {
		return 0, err
	}
	v := reflect.ValueOf(slicePtr).Elem()
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i).Addr().Interface()
		if err := dst.Create(item).Error; err != nil {
			return 0, err
		}
	}
	return int64(v.Len()), nil
}

func openTarget(driver, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres":
		dialector = postgres.Open(dsn)
	default:
		dialector = sqlite.Open(dsn)
	}
	return gorm.Open(dialector, &gorm.Config{})
}

func disableFK(gdb *gorm.DB, driver string) {
	switch driver {
	case "mysql":
		_ = gdb.Exec("SET FOREIGN_KEY_CHECKS = 0").Error
	case "postgres":
		_ = gdb.Exec("SET session_replication_role = replica").Error
	}
}

func enableFK(gdb *gorm.DB, driver string) {
	switch driver {
	case "mysql":
		_ = gdb.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
	case "postgres":
		_ = gdb.Exec("SET session_replication_role = origin").Error
	}
}
