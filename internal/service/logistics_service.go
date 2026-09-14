package service

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"scm/internal/model"
	repo "scm/internal/repo"
)

const (
	sfProdURL    = "https://bspgw.sf-express.com/std/service"
	sfService    = "EXP_RECE_SEARCH_ROUTES"
	sfBatchSize  = 10
	sfApiSuccess = "A1000"
	sfBizSuccess = "S0000"
)

// LogisticsConfig holds SF API credentials.
type LogisticsConfig struct {
	PartnerID string
	Checkword string
}

// LogisticsService queries SF's route API and caches results.
type LogisticsService struct {
	repo *repo.LogisticsRepo
	cfg  LogisticsConfig
	http *http.Client
}

func NewLogisticsService(r *repo.LogisticsRepo, cfg LogisticsConfig) *LogisticsService {
	return &LogisticsService{repo: r, cfg: cfg, http: &http.Client{Timeout: 30 * time.Second}}
}

// QueryOne queries a single tracking number and caches the result.
func (s *LogisticsService) QueryOne(ctx context.Context, t, userID uint, trackingNo string) (*routeResp, error) {
	res, err := s.fetchRoutes([]string{trackingNo}, 1, "")
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, errf(ErrNotFound, "no route found for "+trackingNo)
	}
	r := res[0]
	s.cacheResult(ctx, t, userID, &r)
	return &r, nil
}

// QueryBatch queries multiple tracking numbers (auto-batched, per batch ≤ 10).
func (s *LogisticsService) QueryBatch(ctx context.Context, t, userID uint, trackingNos []string, trackingType int, phoneNo string) ([]routeResp, error) {
	var all []routeResp
	for i := 0; i < len(trackingNos); i += sfBatchSize {
		end := i + sfBatchSize
		if end > len(trackingNos) {
			end = len(trackingNos)
		}
		batch, err := s.fetchRoutes(trackingNos[i:end], trackingType, phoneNo)
		if err != nil {
			return all, err
		}
		for _, r := range batch {
			s.cacheResult(ctx, t, userID, &r)
			all = append(all, r)
		}
		if end < len(trackingNos) {
			time.Sleep(600 * time.Millisecond)
		}
	}
	return all, nil
}

// List returns previously queried tracking numbers.
func (s *LogisticsService) List(ctx context.Context, t, userID uint, page, size int) ([]model.LogisticsQuery, int64, error) {
	return s.repo.List(ctx, t, userID, page, size)
}

// ---- internals ----

type routeResp struct {
	MailNo string      `json:"mailNo"`
	Routes []routeNode `json:"routes"`
}

type routeNode struct {
	AcceptTime         string `json:"acceptTime"`
	AcceptAddress      string `json:"acceptAddress"`
	Remark             string `json:"remark"`
	FirstStatusName    string `json:"firstStatusName"`
	SecondaryStatusName string `json:"secondaryStatusName"`
}

func (s *LogisticsService) sign(msgData, timestamp string) string {
	raw := fmt.Sprintf("%s%s%s", msgData, timestamp, s.cfg.Checkword)
	sum := md5.Sum([]byte(raw))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func (s *LogisticsService) fetchRoutes(trackingNos []string, trackingType int, phoneNo string) ([]routeResp, error) {
	payload := map[string]any{
		"language":       "zh-CN",
		"trackingType":   fmt.Sprintf("%d", trackingType),
		"trackingNumber": trackingNos,
		"methodType":     "1",
	}
	if phoneNo != "" {
		payload["checkPhoneNo"] = phoneNo
	}
	msgData, _ := json.Marshal(payload)
	msgStr := string(msgData)
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	form := url.Values{
		"partnerID":   {s.cfg.PartnerID},
		"requestID":   {uuid.New().String()},
		"serviceCode": {sfService},
		"timestamp":   {ts},
		"msgDigest":   {s.sign(msgStr, ts)},
		"msgData":     {msgStr},
	}
	req, _ := http.NewRequest("POST", sfProdURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "scm-logistics/1.0")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sf api: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var env struct {
		ApiResultCode string `json:"apiResultCode"`
		ApiErrorMsg   string `json:"apiErrorMsg"`
		ApiResultData string `json:"apiResultData"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("sf api decode: %w", err)
	}
	if env.ApiResultCode != sfApiSuccess {
		return nil, fmt.Errorf("sf platform: %s %s", env.ApiResultCode, env.ApiErrorMsg)
	}

	var biz struct {
		Success  bool   `json:"success"`
		ErrorMsg string `json:"errorMsg"`
		MsgData  struct {
			RouteResps []routeResp `json:"routeResps"`
		} `json:"msgData"`
	}
	if err := json.Unmarshal([]byte(env.ApiResultData), &biz); err != nil {
		return nil, fmt.Errorf("sf biz decode: %w", err)
	}
	if !biz.Success {
		return nil, fmt.Errorf("sf biz: %s", biz.ErrorMsg)
	}
	return biz.MsgData.RouteResps, nil
}

func (s *LogisticsService) cacheResult(ctx context.Context, t, userID uint, r *routeResp) {
	routesJSON, _ := json.Marshal(r)
	latest := ""
	if len(r.Routes) > 0 {
		latest = r.Routes[len(r.Routes)-1].SecondaryStatusName
	}
	_ = s.repo.Create(&model.LogisticsQuery{
		TenantID: t, UserID: userID, TrackingNo: r.MailNo,
		LatestStatus: latest, Routes: string(routesJSON), QueryAt: time.Now(),
	})
}