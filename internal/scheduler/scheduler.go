// Package scheduler runs the daily content batch and notification delivery.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/content"
)

type Generator func(context.Context, string) (*content.DailyContent, error)

type MailSender interface {
	Send(context.Context, content.DailyContent, []string) error
}

type UserGenerator func(context.Context, string, string) (*content.DailyContent, error)
type TopicProvider func(context.Context, string) ([]auth.TopicSetting, error)

type Scheduler struct {
	config       config.Config
	generate     Generator
	mail         MailSender
	index        func(context.Context, *content.DailyContent) error
	userGenerate UserGenerator
	topics       TopicProvider
	logger       *slog.Logger
	now          func() time.Time
}

func (s *Scheduler) SetIndexer(index func(context.Context, *content.DailyContent) error) {
	s.index = index
}

func (s *Scheduler) SetUserGenerator(generate UserGenerator) { s.userGenerate = generate }
func (s *Scheduler) SetTopicProvider(provider TopicProvider) { s.topics = provider }

func New(cfg config.Config, generate Generator, mail MailSender, logger *slog.Logger) (*Scheduler, error) {
	if generate == nil {
		return nil, fmt.Errorf("content generator is required")
	}

	if logger == nil {
		logger = slog.Default()
	}

	location, err := time.LoadLocation(cfg.ScheduleTZ)

	if err != nil {
		return nil, fmt.Errorf("load schedule timezone: %w", err)
	}

	if cfg.ScheduleTZ == "" {
		location = time.Local
	}

	return &Scheduler{config: cfg, generate: generate, mail: mail, logger: logger, now: func() time.Time { return time.Now().In(location) }}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	for {
		next := nextSixAM(s.now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			if err := s.RunOnce(ctx); err != nil {
				s.logger.ErrorContext(ctx, "daily batch failed", "error", err)
			}
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	if s.topics != nil && s.userGenerate != nil {
		date := s.now().Format("2006-01-02")
		settings, err := s.topics(ctx, date)
		if err != nil {
			return fmt.Errorf("load topic settings: %w", err)
		}
		for _, setting := range settings {
			daily, err := s.userGenerate(ctx, setting.UserID, setting.Topic)
			if err != nil {
				return fmt.Errorf("generate user daily content: %w", err)
			}
			s.logger.InfoContext(ctx, "user daily content generated", "user_id", setting.UserID, "date", daily.Date, "theme", daily.Theme)
			if s.index != nil {
				if err := s.index(ctx, daily); err != nil {
					return fmt.Errorf("index user daily content: %w", err)
				}
			}
		}
		if len(settings) > 0 {
			return nil
		}
	}
	daily, err := s.generate(ctx, s.config.DailyTopic)
	if err != nil {
		return fmt.Errorf("generate daily content: %w", err)
	}

	s.logger.InfoContext(ctx, "daily content generated", "date", daily.Date, "theme", daily.Theme)
	if s.index != nil {
		if err := s.index(ctx, daily); err != nil {
			return fmt.Errorf("index daily content: %w", err)
		}
	}
	if len(s.config.EmailRecipients) == 0 || s.mail == nil {
		s.logger.InfoContext(ctx, "daily email skipped", "reason", "no recipients configured")
		return nil
	}

	if err := s.mail.Send(ctx, *daily, s.config.EmailRecipients); err != nil {
		return fmt.Errorf("send daily email: %w", err)
	}

	return nil
}

func nextSixAM(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), 6, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

type SMTPMailer struct{ config config.Config }

func NewSMTPMailer(cfg config.Config) *SMTPMailer { return &SMTPMailer{config: cfg} }

func (m *SMTPMailer) Send(ctx context.Context, daily content.DailyContent, recipients []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(m.config.SMTPHost) == "" {
		return fmt.Errorf("SMTP_HOST is required")
	}

	if strings.TrimSpace(m.config.EmailFrom) == "" {
		return fmt.Errorf("EMAIL_FROM is required")
	}

	if m.config.SMTPPort <= 0 {
		return fmt.Errorf("SMTP_PORT must be positive")
	}

	subject := "Today Japanese · " + daily.Date + " · " + daily.Theme
	body := buildBody(daily)
	message := "From: " + m.config.EmailFrom + "\r\n" + "To: " + strings.Join(recipients, ", ") + "\r\n" + "Subject: " + subject + "\r\n" + "Content-Type: text/plain; charset=UTF-8\r\n\r\n" + body
	auth := smtp.Auth(nil)

	if m.config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", m.config.SMTPUsername, m.config.SMTPPassword, m.config.SMTPHost)
	}

	address := fmt.Sprintf("%s:%d", m.config.SMTPHost, m.config.SMTPPort)

	return smtp.SendMail(address, auth, m.config.EmailFrom, recipients, []byte(message))
}

func buildBody(daily content.DailyContent) string {
	var builder strings.Builder

	fmt.Fprintf(&builder, "오늘의 일본어: %s (%s)\n\n", daily.Theme, daily.Level)

	builder.WriteString("단어\n")

	for _, word := range daily.Words {
		fmt.Fprintf(&builder, "- %s (%s): %s\n  예문: %s / %s\n", word.Word, word.Reading, word.Meaning, word.Example, word.ExampleTranslation)
	}

	builder.WriteString("\n문장\n")

	for _, sentence := range daily.Sentences {
		fmt.Fprintf(&builder, "- %s (%s): %s\n", sentence.Japanese, sentence.Reading, sentence.Translation)
	}

	fmt.Fprintf(&builder, "\n문법: %s (%s)\n%s\n\n%s\n", daily.Grammar.Pattern, daily.Grammar.Meaning, daily.Grammar.Explanation, daily.MomentsText)

	return builder.String()
}
