package request

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/AVGsync/invoice-keeper/backend/internal/model"
	"github.com/google/uuid"
)

var (
	// actionParamRe — точный код или префикс с '*': "recurring.update", "recurring.*".
	actionParamRe = regexp.MustCompile(`^[a-z_]+(\.[a-z_]+)*(\.\*)?$`)
	// entityTypeRe — тип сущности.
	entityTypeRe = regexp.MustCompile(`^[a-z_]{1,64}$`)
)

// ParseAuditFilter разбирает query-параметры лент журнала.
func ParseAuditFilter(q url.Values, loc *time.Location) (model.AuditFilter, error) {
	var f model.AuditFilter

	actorIDs := multiValues(q, "actor_id")
	for _, id := range actorIDs {
		if _, err := uuid.Parse(id); err != nil {
			return f, errors.New("actor_id must be a UUID")
		}
	}
	f.ActorIDs = actorIDs

	actions := multiValues(q, "action")
	for _, a := range actions {
		if !actionParamRe.MatchString(a) {
			return f, errors.New("action is invalid")
		}
	}
	f.Actions = actions

	entityTypes := multiValues(q, "entity_type")
	for _, et := range entityTypes {
		if !entityTypeRe.MatchString(et) {
			return f, errors.New("entity_type is invalid")
		}
	}
	f.EntityTypes = entityTypes

	if eid := strings.TrimSpace(q.Get("entity_id")); eid != "" {
		if utf8.RuneCountInString(eid) > 128 {
			return f, errors.New("entity_id must be at most 128 characters")
		}
		f.EntityID = eid
	}

	channels := multiValues(q, "channel")
	for _, c := range channels {
		switch c {
		case "web", "miniapp", "bot", "system":
		default:
			return f, errors.New("channel must be one of web, miniapp, bot, system")
		}
	}
	f.Channels = channels

	if outcome := strings.TrimSpace(q.Get("outcome")); outcome != "" {
		if outcome != "success" && outcome != "failure" {
			return f, errors.New("outcome must be success or failure")
		}
		f.Outcome = outcome
	}

	fromT, fromCmp, err := parseAuditBound(q.Get("from"), loc, false, "from must be YYYY-MM-DD or RFC3339")
	if err != nil {
		return f, err
	}
	toT, toCmp, err := parseAuditBound(q.Get("to"), loc, true, "to must be YYYY-MM-DD or RFC3339")
	if err != nil {
		return f, err
	}
	if fromT != nil && toT != nil && fromCmp.After(toCmp) {
		return f, errors.New("from must be <= to")
	}
	f.From = fromT
	f.To = toT

	query, err := parseQuery(q)
	if err != nil {
		return f, err
	}
	f.Query = query

	order := strings.TrimSpace(q.Get("order"))
	if order == "" {
		order = "desc"
	}
	if order != "asc" && order != "desc" {
		return f, errors.New("order must be asc or desc")
	}
	f.Order = order

	if f.Limit, f.Offset, err = ParsePage(q); err != nil {
		return f, err
	}

	return f, nil
}

// parseAuditBound разбирает границу времени: YYYY-MM-DD в зоне loc или RFC3339.
func parseAuditBound(raw string, loc *time.Location, isTo bool, onErr string) (*time.Time, time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, time.Time{}, nil
	}
	if loc == nil {
		loc = time.UTC
	}
	if len(raw) == 10 {
		if d, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
			t := d
			if isTo {
				t = d.AddDate(0, 0, 1)
			}
			return &t, d, nil
		}
	}
	if inst, err := time.Parse(time.RFC3339, raw); err == nil {
		t := inst
		if isTo {
			t = inst.Add(time.Nanosecond)
		}
		return &t, inst, nil
	}
	return nil, time.Time{}, errors.New(onErr)
}
