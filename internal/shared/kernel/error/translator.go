package apperror

import (
	"context"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"

	"github.com/kadekutama/go-template/internal/shared/locale"
)

// localeKey is the context key carrying the request locale.
type localeKey struct{}

// WithLocale returns a context rendering Translator messages in the given
// BCP-47 tag (e.g. "en", "id"). Unknown tags fall back to English.
func WithLocale(ctx context.Context, tag string) context.Context {
	return context.WithValue(ctx, localeKey{}, tag)
}

// localeFrom reports the locale tag stored in ctx, defaulting to English.
func localeFrom(ctx context.Context) string {
	tag, _ := ctx.Value(localeKey{}).(string)
	if tag == "" {
		return "en"
	}
	return tag
}

// Translator renders registered codes through the embedded catalogs.
type Translator struct {
	bundle *i18n.Bundle
}

// NewTranslator loads en.yaml/id.yaml from the embedded filesystem.
func NewTranslator() (*Translator, error) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)
	for _, name := range []string{"en.yaml", "id.yaml"} {
		data, err := locale.FS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		if _, err := bundle.ParseMessageFileBytes(data, name); err != nil {
			return nil, err
		}
	}
	return &Translator{bundle: bundle}, nil
}

// Translate renders code for the context locale. Unknown codes or missing
// messages fall back to the AppError message so callers never blank out.
func (t *Translator) Translate(ctx context.Context, err *AppError) string {
	if err == nil {
		return ""
	}
	if t == nil || t.bundle == nil {
		return err.Message
	}
	localizer := i18n.NewLocalizer(t.bundle, localeFrom(ctx), "en")
	msg, locErr := localizer.Localize(&i18n.LocalizeConfig{MessageID: string(err.Code)})
	if locErr != nil || msg == "" {
		return err.Message
	}
	return msg
}
