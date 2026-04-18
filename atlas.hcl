data "external_schema" "gorm" {
  program = [
    "go", "run", "-mod=mod",
    "ariga.io/atlas-provider-gorm",
    "load",
    "--path", "./internal/models",
    "--dialect", "postgres",
  ]
}

env "gorm" {
  src = data.external_schema.gorm.url

  # A separate empty DB for Atlas to use as a scratch space when computing diffs.
  # Never point this at your real database — Atlas drops and recreates schemas freely here.
  # Alternative if you have Docker: dev = "docker://postgres/16/dev"
  dev = "postgres://obrien:admin123@localhost:5432/proctura_dev_db"

  migration {
    dir = "file://migrations"
  }

  format {
    migrate {
      diff = "{{ sql . \"  \" }}"
    }
  }
}

