package meta

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

// EventName is fixed to Meta's standard Contact event for deliberate user enquiries.
const EventName = "Contact"

// LegacyManualEventName remains readable in historical event logs and test filters.
const LegacyManualEventName = "WhatsAppConsultClick"

// AutoRedirectEventName keeps timer-driven consultations separate from deliberate clicks in Meta.
const AutoRedirectEventName = "WhatsAppAutoRedirect"

// GraphAPIVersion is application-owned so operators cannot move individual
// accounts onto an untested Graph contract.
const GraphAPIVersion = "v26.0"

var idPattern = regexp.MustCompile(`^[0-9]{1,32}$`)
var versionPattern = regexp.MustCompile(`^v[0-9]{2,3}\.0$`)

// Connection groups one ad account and its Pixels. Insights fields remain only
// in historical database migrations and are deliberately absent from the API.
type Connection struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	AccountID  string    `json:"account_id"`
	APIVersion string    `json:"api_version"`
	CreatedAt  time.Time `json:"created_at"`
}
type ConnectionInput struct{ Connection }

const connectionColumns = "id,name,account_id,api_version,created_at"

func scanConnection(row pgx.Row) (c Connection, e error) {
	e = row.Scan(&c.ID, &c.Name, &c.AccountID, &c.APIVersion, &c.CreatedAt)
	return
}
func validateConnection(c Connection) error {
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 120 {
		return fmt.Errorf("请填写接入名称（最多 120 字节）")
	}
	if !idPattern.MatchString(c.AccountID) {
		return fmt.Errorf("广告账户 ID 必须为数字，不包含 act_ 前缀")
	}
	return nil
}

type Service struct {
	Core      *runtime.Core
	Client    *Client
	lifecycle sync.Mutex
	cancel    context.CancelFunc
	closed    bool
	workers   sync.WaitGroup
}

func New(core *runtime.Core) *Service { return &Service{Core: core, Client: NewClient()} }
func (s *Service) Connection(ctx context.Context, id int64) (Connection, error) {
	return scanConnection(s.Core.DB.QueryRow(ctx, "SELECT "+connectionColumns+" FROM meta_connections WHERE id=$1", id))
}
func (s *Service) Connections(ctx context.Context) ([]Connection, error) {
	rows, e := s.Core.DB.Query(ctx, "SELECT "+connectionColumns+" FROM meta_connections ORDER BY id DESC")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Connection{}
	for rows.Next() {
		c, e := scanConnection(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
