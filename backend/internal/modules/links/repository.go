package links

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Scan(row pgx.Row) (l Link, e error) {
	e = row.Scan(&l.ID, &l.Code, &l.Name, &l.TargetURL, &l.Enabled, &l.CampaignID, &l.AdsetID, &l.AdID, &l.Channel, &l.CreatedAt, &l.ExpiresAt, &l.Mode, &l.LandingBrand, &l.LandingTitle, &l.LandingDescription, &l.LandingDetails, &l.LandingDelay, &l.MetaConnectionID, &l.AttributionMode, &l.LegacyCampaignParam, &l.MetaPixelID)
	return
}

// Repository owns link persistence and its atomic audit record.
type Repository struct{ DB *pgxpool.Pool }

func (r Repository) ByCode(ctx context.Context, code string) (Link, error) {
	return Scan(r.DB.QueryRow(ctx, "SELECT "+Columns+" FROM short_links WHERE code=$1", code))
}
func (r Repository) ByID(ctx context.Context, id int64) (Link, error) {
	return Scan(r.DB.QueryRow(ctx, "SELECT "+Columns+" FROM short_links WHERE id=$1", id))
}
func (r Repository) List(ctx context.Context) ([]Link, error) {
	rows, e := r.DB.Query(ctx, "SELECT "+Columns+" FROM short_links ORDER BY id DESC LIMIT 10000")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Link{}
	for rows.Next() {
		l, e := Scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (r Repository) Save(ctx context.Context, l Link, update bool, actor string) (Link, error) {
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return Link{}, e
	}
	defer tx.Rollback(ctx)
	if l.AttributionMode == "" {
		l.AttributionMode = "bound"
	}
	args := []any{l.Code, l.Name, l.TargetURL, l.Enabled, l.CampaignID, l.AdsetID, l.AdID, l.Channel, l.ExpiresAt, l.Mode, l.LandingBrand, l.LandingTitle, l.LandingDescription, l.LandingDetails, l.LandingDelay, l.MetaConnectionID, l.AttributionMode, l.LegacyCampaignParam, l.MetaPixelID}
	query := "INSERT INTO short_links(code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,expires_at,mode,landing_brand,landing_title,landing_description,landing_details,landing_delay,meta_connection_id,attribution_mode,legacy_campaign_param,meta_pixel_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19) RETURNING " + Columns
	action := "link.create"
	if update {
		action = "link.update"
		args = append(args, l.ID)
		query = "UPDATE short_links SET code=$1,name=$2,target_url=$3,enabled=$4,campaign_id=$5,adset_id=$6,ad_id=$7,channel=$8,expires_at=$9,mode=$10,landing_brand=$11,landing_title=$12,landing_description=$13,landing_details=$14,landing_delay=$15,meta_connection_id=$16,attribution_mode=$17,legacy_campaign_param=$18,meta_pixel_id=$19 WHERE id=$20 RETURNING " + Columns
	}

	saved, e := Scan(tx.QueryRow(ctx, query, args...))
	if e != nil {
		return Link{}, e
	}
	payload, e := json.Marshal(saved)
	if e != nil {
		return Link{}, e
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, payload); e != nil {
		return Link{}, e
	}
	return saved, tx.Commit(ctx)
}
