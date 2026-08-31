package service

import (
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/pkg/authx"
	"scm/internal/repository"
)

// RBACService owns tenants, users, the permission catalog and enforcement.
// The catalog (modules/permissions/roles/matrix) lives in tables; the
// route→permission binding is code (routes are code), resolved against the
// DB-stored catalog at startup and cached.
type RBACService struct {
	tenants *repository.TenantRepo
	users   *repository.UserRepo
	roles   *repository.RoleRepo
	modules *repository.ModuleRepo
	perms   *repository.PermissionRepo
	userRol *repository.UserRoleRepo
	db      *gorm.DB

	mu    sync.RWMutex
	cache map[string]map[string]bool // roleCode -> permCode -> granted
}

func NewRBACService(d RBACDeps) *RBACService {
	return &RBACService{
		tenants: d.Tenants, users: d.Users, roles: d.Roles,
		modules: d.Modules, perms: d.Perms, userRol: d.UserRoles, db: d.DB,
	}
}

// RBACDeps groups the repositories an RBACService needs.
type RBACDeps struct {
	Tenants   *repository.TenantRepo
	Users     *repository.UserRepo
	Roles     *repository.RoleRepo
	Modules   *repository.ModuleRepo
	Perms     *repository.PermissionRepo
	UserRoles *repository.UserRoleRepo
	DB        *gorm.DB
}

// ---- permission cache & enforcement ----

// RefreshCache reloads the role→permission matrix from tables.
func (s *RBACService) RefreshCache() error {
	var rps []model.RolePermission
	if err := s.db.Find(&rps).Error; err != nil {
		return err
	}
	permByID := map[uint]string{}
	var perms []model.Permission
	if err := s.db.Find(&perms).Error; err != nil {
		return err
	}
	for _, p := range perms {
		permByID[p.ID] = p.Code
	}
	roles, err := s.roles.All()
	if err != nil {
		return err
	}
	roleByID := map[uint]string{}
	for _, r := range roles {
		roleByID[r.ID] = r.Code
	}
	cache := map[string]map[string]bool{}
	for _, rp := range rps {
		rc, ok := roleByID[rp.RoleID]
		if !ok {
			continue
		}
		if cache[rc] == nil {
			cache[rc] = map[string]bool{}
		}
		cache[rc][permByID[rp.PermissionID]] = true
	}
	s.mu.Lock()
	s.cache = cache
	s.mu.Unlock()
	return nil
}

// HasPerm reports whether any of the actor's roles grants the permission.
func (s *RBACService) HasPerm(a *authx.Actor, perm string) bool {
	if a == nil {
		return false
	}
	for _, r := range a.Roles {
		if r == model.RoleAdmin {
			return true
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range a.Roles {
		if s.cache[r][perm] {
			return true
		}
	}
	return false
}

// routePerm binds an HTTP route to a permission code.
type routePerm struct{ method, pattern, perm string }

// routePerms is the enforcement table; :seg matches one path segment.
var routePerms = []routePerm{
	{"GET", "/materials", "material:view"}, {"POST", "/materials", "material:manage"},
	{"PUT", "/materials/:id", "material:manage"}, {"DELETE", "/materials/:id", "material:manage"},
	{"GET", "/suppliers", "supplier:view"}, {"POST", "/suppliers", "supplier:manage"},
	{"PUT", "/suppliers/:id", "supplier:manage"}, {"DELETE", "/suppliers/:id", "supplier:manage"},
	{"PUT", "/suppliers/:id/audit", "supplier:audit"},
	{"GET", "/customers", "customer:view"}, {"POST", "/customers", "customer:manage"},
	{"PUT", "/customers/:id", "customer:manage"}, {"DELETE", "/customers/:id", "customer:manage"},
	{"GET", "/warehouses", "warehouse:view"}, {"POST", "/warehouses", "warehouse:manage"},
	{"PUT", "/warehouses/:id", "warehouse:manage"}, {"DELETE", "/warehouses/:id", "warehouse:manage"},
	{"GET", "/locations", "warehouse:view"}, {"POST", "/locations", "warehouse:manage"},
	{"PUT", "/locations/:id", "warehouse:manage"}, {"DELETE", "/locations/:id", "warehouse:manage"},
	{"GET", "/po", "po:view"}, {"GET", "/po/:id", "po:view"}, {"POST", "/po", "po:create"},
	{"PUT", "/po/:id", "po:edit"}, {"PUT", "/po/:id/status", "po:approve"},
	{"DELETE", "/po/:id", "po:delete"}, {"POST", "/po/:id/receive", "po:receive"},
	{"GET", "/po/:id/receipts", "po:view"},
	{"GET", "/so", "so:view"}, {"GET", "/so/:id", "so:view"}, {"POST", "/so", "so:create"},
	{"PUT", "/so/:id/approve", "so:approve"}, {"PUT", "/so/:id/cancel", "so:cancel"},
	{"DELETE", "/so/:id", "so:delete"},
	{"GET", "/inventory/stock", "stock:view"},
	{"POST", "/inventory/move-in", "inv:move"}, {"POST", "/inventory/move-out", "inv:move"},
	{"GET", "/inventory/orders", "stock:view"}, {"GET", "/inventory/orders/:id", "stock:view"},
	{"DELETE", "/inventory/orders/:id", "inv:order:delete"},
	{"GET", "/inventory/logs", "inv:logs:view"},
	{"GET", "/planning/demands", "demand:view"}, {"GET", "/planning/demands/:id", "demand:view"},
	{"POST", "/planning/demands", "demand:manage"}, {"PUT", "/planning/demands/:id", "demand:manage"},
	{"DELETE", "/planning/demands/:id", "demand:manage"},
	{"GET", "/planning/mrp", "mrp:view"}, {"GET", "/planning/mrp/:id", "mrp:view"},
	{"DELETE", "/planning/mrp/:id", "mrp:compute"},
	{"POST", "/planning/mrp/compute", "mrp:compute"}, {"POST", "/planning/mrp/:id/convert", "mrp:convert"},
	{"GET", "/users", "user:manage"}, {"POST", "/users", "user:manage"},
	{"PUT", "/users/:id", "user:manage"}, {"DELETE", "/users/:id", "user:manage"},
	{"GET", "/tenants", "tenant:manage"}, {"POST", "/tenants", "tenant:manage"},
	{"PUT", "/tenants/:id", "tenant:manage"},
	{"GET", "/rbac/catalog", "perms:view"},
}

// PermForRoute resolves the permission guarding a request; empty means any
// authenticated user.
func PermForRoute(method, path string) string {
	path = strings.TrimSuffix(path, "/")
	segs := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, rp := range routePerms {
		if rp.method != method && rp.method != "*" {
			continue
		}
		ps := strings.Split(strings.TrimPrefix(rp.pattern, "/"), "/")
		if len(ps) != len(segs) {
			continue
		}
		ok := true
		for i, p := range ps {
			if strings.HasPrefix(p, ":") {
				continue
			}
			if p != segs[i] {
				ok = false
				break
			}
		}
		if ok {
			return rp.perm
		}
	}
	return ""
}

// PermsForActor lists every permission code the actor holds (menu gating).
func (s *RBACService) PermsForActor(a *authx.Actor) []string {
	all, err := s.perms.All()
	if err != nil {
		return nil
	}
	var codes []string
	for _, p := range all {
		if s.HasPerm(a, p.Code) {
			codes = append(codes, p.Code)
		}
	}
	return codes
}

// Check enforces the route permission for the actor.
func (s *RBACService) Check(a *authx.Actor, method, path string) error {
	path = strings.TrimPrefix(path, "/api/v1")
	perm := PermForRoute(method, path)
	if perm == "" {
		return nil
	}
	if !s.HasPerm(a, perm) {
		return errf(ErrForbidden, "permission denied: "+perm)
	}
	return nil
}

// ---- tenant management (platform scope) ----

func (s *RBACService) ListTenants(in PageInput) ([]model.Tenant, int64, error) {
	var (
		out   []model.Tenant
		total int64
	)
	if err := s.tenants.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *RBACService) CreateTenant(t *model.Tenant) error {
	if t.Code == "" || t.Name == "" {
		return errorsBadRequest("code/name are required")
	}
	if existing, _ := s.tenants.GetByCode(t.Code); existing != nil {
		return errf(ErrConflict, "tenant code already exists")
	}
	return s.tenants.Create(t)
}

// UpdateTenant edits name/plan/status (suspend lives here).
func (s *RBACService) UpdateTenant(id uint, t *model.Tenant) error {
	t.ID = id
	return s.tenants.Update(t)
}

func (s *RBACService) GetTenantByCode(code string) (*model.Tenant, error) {
	return s.tenants.GetByCode(code)
}

// ---- user management (tenant scope) ----

func (s *RBACService) ListUsers(t uint, in PageInput) ([]model.User, int64, error) {
	var (
		out   []model.User
		total int64
	)
	if err := s.users.List(t, repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// CreateUserInput is the payload for provisioning a user with roles.
type CreateUserInput struct {
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	Name       string   `json:"name"`
	Status     int8     `json:"status"`
	RoleCodes  []string `json:"role_codes"`
	TenantCode string   `json:"tenant_code"` // platform-scope bootstrap: create the user in another tenant
}

func (s *RBACService) CreateUser(t uint, in CreateUserInput) (*model.User, error) {
	if in.Username == "" || in.Password == "" {
		return nil, errorsBadRequest("username/password are required")
	}
	// Platform admins may provision users into any tenant by tenant_code;
	// everyone else only manages users of their own tenant.
	if in.TenantCode != "" {
		target, err := s.tenants.GetByCode(in.TenantCode)
		if err != nil {
			return nil, err
		}
		if target == nil {
			return nil, errorsBadRequest("unknown tenant_code")
		}
		t = target.ID
	}
	if existing, _ := s.users.GetByTenantUsername(t, in.Username); existing != nil {
		return nil, errf(ErrConflict, "username already exists in this tenant")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	u := &model.User{TenantID: t, Username: in.Username, PasswordHash: string(hash), Name: in.Name, Status: 1}
	if err := s.users.Create(u); err != nil {
		return nil, err
	}
	if err := s.setRoleCodes(u.ID, in.RoleCodes); err != nil {
		return nil, err
	}
	return u, nil
}

// UpdateUser edits name/status/password(optional)/roles.
func (s *RBACService) UpdateUser(t, id uint, in CreateUserInput) (*model.User, error) {
	u, err := s.users.Get(t, id)
	if u == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	if in.Name != "" {
		u.Name = in.Name
	}
	if in.Status != 0 {
		u.Status = in.Status
	}
	if in.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = string(hash)
	}
	if err := s.users.Update(t, u); err != nil {
		return nil, err
	}
	if len(in.RoleCodes) > 0 {
		if err := s.setRoleCodes(u.ID, in.RoleCodes); err != nil {
			return nil, err
		}
	}
	return u, nil
}

func (s *RBACService) DeleteUser(t, id uint) error {
	return s.users.Delete(t, id)
}

// UserRoles lists role codes of a user (for the admin UI).
func (s *RBACService) UserRoles(userID uint) ([]string, error) {
	return s.userRol.RoleCodesForUser(userID)
}

func (s *RBACService) setRoleCodes(userID uint, codes []string) error {
	all, err := s.roles.All()
	if err != nil {
		return err
	}
	byCode := map[string]uint{}
	for _, r := range all {
		byCode[r.Code] = r.ID
	}
	var ids []uint
	for _, c := range codes {
		if id, ok := byCode[c]; ok {
			ids = append(ids, id)
		}
	}
	return s.userRol.SetRoles(userID, ids)
}

// Catalog returns modules+permissions+roles for the admin UI.
func (s *RBACService) Catalog() (map[string]any, error) {
	modules, err := s.modules.All()
	if err != nil {
		return nil, err
	}
	perms, err := s.perms.All()
	if err != nil {
		return nil, err
	}
	roles, err := s.roles.All()
	if err != nil {
		return nil, err
	}
	return map[string]any{"modules": modules, "permissions": perms, "roles": roles}, nil
}

