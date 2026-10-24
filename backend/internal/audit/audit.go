// Package audit ведёт журнал действий пользователей (веб, Mini App, бот, система).
//
// Запись best-effort и синхронная: выполняется ПОСЛЕ успешной мутации, в том же
// запросе, с context.WithoutCancel и таймаутом (см. Logger.Record). Ошибка записи
// только логируется и не откатывает основное действие — журнал действий это не
// финансовый леджер, а репозитории не делят транзакцию. Метаданные запроса
// (актор, канал, IP, UA, request_id) кладёт в контекст middleware/бот через
// WithMeta, конкретное событие описывает Entry.
package audit

import (
	"context"
	"reflect"
)

// Каналы и исходы — ровно значения CHECK-ограничений таблицы audit_log.
const (
	ChannelWeb     = "web"
	ChannelMiniApp = "miniapp"
	ChannelBot     = "bot"
	ChannelSystem  = "system"
)

const (
	OutcomeSuccess = "success"
	OutcomeFailure = "failure"
)

// Meta — метаданные запроса, общие для всех событий одного запроса. Кладётся в
// контекст middleware (HTTP) или ботом (WithMeta), берётся Logger при записи.
type Meta struct {
	ActorID   string
	ActorRole string
	Channel   string
	IP        string
	UserAgent string
	RequestID string
}

type metaKey struct{}

// WithMeta кладёт метаданные запроса в контекст.
func WithMeta(ctx context.Context, m Meta) context.Context {
	return context.WithValue(ctx, metaKey{}, m)
}

// MetaFrom достаёт метаданные запроса из контекста.
func MetaFrom(ctx context.Context) (Meta, bool) {
	m, ok := ctx.Value(metaKey{}).(Meta)
	return m, ok
}

// Change — изменение одного поля «было → стало».
type Change struct {
	From any `json:"from"`
	To   any `json:"to"`
}

// Entry описывает одно событие журнала. Outcome "" трактуется как success.
// ActorID/ActorRole переопределяют Meta (вход, бот), пустые — берутся из Meta.
type Entry struct {
	Action     string
	Outcome    string
	EntityType string
	EntityID   string
	ActorID    string
	ActorRole  string
	Changes    map[string]Change
	Payload    map[string]any
}

// Recorder — точка записи, которую получают сервисы. Record не возвращает ошибку:
// журнал не должен ломать основное действие.
type Recorder interface {
	Record(ctx context.Context, e Entry)
}

// Nop — рекордер по умолчанию: ничего не пишет. Ставится во всех сервисах и
// тестах, пока apiserver не подключит настоящий Logger.
type Nop struct{}

func (Nop) Record(context.Context, Entry) {}

// Repository — запись строки журнала в хранилище.
type Repository interface {
	Insert(ctx context.Context, row Row) error
}

// Row — строка таблицы audit_log. Указатели — для NULL-колонок; Changes/Payload —
// уже сериализованный JSON (nil = NULL).
type Row struct {
	ActorID    *string
	ActorRole  *string
	Channel    string
	Action     string
	Outcome    string
	EntityType *string
	EntityID   *string
	Changes    []byte
	Payload    []byte
	IP         *string
	UserAgent  *string
	RequestID  *string
}

// DiffBuilder собирает changes, включая только реально изменившиеся поля.
type DiffBuilder struct {
	m map[string]Change
}

// Diff создаёт билдер изменений.
func Diff() *DiffBuilder {
	return &DiffBuilder{m: make(map[string]Change)}
}

// Add добавляет поле, только если from и to различаются.
func (d *DiffBuilder) Add(field string, from, to any) *DiffBuilder {
	if !reflect.DeepEqual(from, to) {
		d.m[field] = Change{From: from, To: to}
	}
	return d
}

// Map возвращает собранные изменения или nil, если изменений нет.
func (d *DiffBuilder) Map() map[string]Change {
	if len(d.m) == 0 {
		return nil
	}
	return d.m
}
