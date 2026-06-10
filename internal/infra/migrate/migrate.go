// Package migrate başlangıçta otomatik migration çalıştırır.
// Belirtilen dizindeki *.up.sql dosyalarını ad sırasıyla yürütür.
// Idempotent: CREATE TABLE IF NOT EXISTS kullanıldığı için tekrar çalıştırmak güvenlidir.
package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Run migDir içindeki tüm *.up.sql dosyalarını ad sırasıyla yürütür.
func Run(ctx context.Context, pool *pgxpool.Pool, migDir string, log *slog.Logger) error {
	entries, err := os.ReadDir(migDir)
	if err != nil {
		return fmt.Errorf("migrate: dizin okunamadı %s: %w", migDir, err)
	}

	// Sadece .up.sql dosyalarını sıralı al
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	var applied int
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(migDir, f))
		if err != nil {
			return fmt.Errorf("migrate: %s okunamadı: %w", f, err)
		}

		sql := strings.TrimSpace(string(data))
		if sql == "" {
			continue
		}

		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migrate: %s hatası: %w", f, err)
		}

		log.Info("migration uygulandı", "file", f)
		applied++
	}

	if applied > 0 {
		log.Info("migration tamamlandı", "count", applied)
	} else {
		log.Info("uygulanacak migration yok")
	}
	return nil
}
