package reminder_test

import (
	"testing"

	"github.com/AVGsync/invoice-keeper/backend/internal/reminder"
	"github.com/AVGsync/invoice-keeper/backend/internal/reminder/remindertest"
)

func TestMemoryStoreSourceContract(t *testing.T) {
	remindertest.RunSourceContract(t, func(t *testing.T, fx remindertest.Fixtures) reminder.OccurrenceSource {
		return reminder.NewMemoryStore(fx.Occurrences, nil)
	})
}

func TestMemoryStorePayerContract(t *testing.T) {
	remindertest.RunPayerContract(t, func(t *testing.T, fx remindertest.Fixtures) reminder.Payer {
		return reminder.NewMemoryStore(fx.Occurrences, reminder.StaticAdmins(fx.Admin))
	})
}
