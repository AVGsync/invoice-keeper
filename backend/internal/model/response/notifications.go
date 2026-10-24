package response

import "time"

type UserNotificationSettingsResponse struct {
	UserID        string    `json:"user_id"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	EmailEnabled  bool      `json:"email_enabled"`
	DaysBeforeDue int       `json:"days_before_due"`
	SendTime      string    `json:"send_time"`
	Timezone      string    `json:"timezone"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
