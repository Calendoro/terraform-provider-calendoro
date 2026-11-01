terraform {
  required_providers {
    calendoro = { source = "calendoro/calendoro" }
  }
}

provider "calendoro" {} # reads CALENDORO_API_KEY

resource "calendoro_event_type" "intro" {
  slug             = "intro"
  title            = "Intro call"
  duration_minutes = 30
  published        = true
}

resource "calendoro_webhook" "bookings" {
  url         = "https://example.com/hooks/calendoro"
  events_json = jsonencode(["booking.created", "booking.cancelled"])
}

output "webhook_secret" {
  value     = calendoro_webhook.bookings.secret
  sensitive = true
}
