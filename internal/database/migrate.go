package database

import (
	"context"
	"fmt"
	"log"
	"os"

	"ariga.io/atlas/atlasexec"
)

func RunMigrations(migrationsDir string, dbURL string) error {
	workdir, err := atlasexec.NewWorkingDir(
		atlasexec.WithMigrations(
			os.DirFS(migrationsDir),
		),
	)
	if err != nil {
		return fmt.Errorf("load migrations directory: %w", err)
	}
	defer workdir.Close()

	client, err := atlasexec.NewClient(workdir.Path(), "atlas")
	if err != nil {
		return fmt.Errorf("initialize atlas client: %w", err)
	}

	res, err := client.MigrateApply(context.Background(), &atlasexec.MigrateApplyParams{
		URL: dbURL,
	})
	if err != nil {
		return fmt.Errorf("atlas migrate apply: %w", err)
	}

	log.Printf("[db] applied %d migration(s)", len(res.Applied))
	return nil
}
