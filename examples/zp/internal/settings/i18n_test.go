package settings_test

import (
	"testing"

	"github.com/epistemer/i18n"
	"github.com/epistemer/i18n/i18ntest"

	"zp/internal/settings"
)

// The labels App.Labels publishes, and i18n's generator turns into Go, are
// i18n keys: the translator registers
// them, a translation shows in its locale, and L10n finds them by path.
func TestLabels(t *testing.T) {
	tr := i18ntest.New(t, i18n.Locale{Code: "en", NativeName: "English"})
	if err := tr.CreateLocale(i18n.Locale{Code: "ar", NativeName: "العربية", RTL: true}); err != nil {
		t.Fatal(err)
	}
	for key, text := range map[string]string{
		"settings.server.port":     "منفذ الاستماع",
		"settings.log.level.debug": "تصحيح",
		"settings.scheduler.retry": "سياسة إعادة المحاولة",
	} {
		if err := tr.Set("ar", key, text); err != nil {
			t.Fatalf("Set %s: %v", key, err)
		}
	}
	ar, en := i18ntest.Context(t, tr, "ar"), i18ntest.Context(t, tr, "en")

	if got := settings.L10nServerPort.T(ar); got != "منفذ الاستماع" {
		t.Errorf("L10nServerPort in ar: %q", got)
	}
	if got := settings.L10nServerPort.T(en); got != "Listen port" {
		t.Errorf("L10nServerPort in en: %q", got)
	}
	for path, want := range map[string]string{
		"server.port":     "منفذ الاستماع",
		"scheduler.retry": "سياسة إعادة المحاولة",
		"log.level.debug": "تصحيح",
		"log.level.info":  "info",
		"database":        "Database connection",
	} {
		l := settings.L10n(path)
		if l == nil {
			t.Errorf("L10n(%q) is nil", path)
			continue
		}
		if got := l.T(ar); got != want {
			t.Errorf("L10n(%q) in ar: %q, want %q", path, got, want)
		}
	}
	if settings.L10n("server.nope") != nil {
		t.Error("L10n of an unknown path isn't nil")
	}
}
