package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigAppliesEnvironmentOverrides(t *testing.T) {
	t.Setenv("SERVER_PORT", "5500")
	t.Setenv("DATABASE_DSN", "postgres://postgres:postgres@localhost:5432/postgres")
	t.Setenv("CORS_ORIGINS", "http://localhost:3301,http://127.0.0.1:3301")

	viper.Reset()
	t.Cleanup(func() {
		viper.Reset()
		AppConfig = nil
	})

	require.NoError(t, LoadConfig("./definitely-missing"))
	require.NotNil(t, AppConfig)
	require.Equal(t, "5500", AppConfig.Server.Port)
	require.Equal(t, "postgres://postgres:postgres@localhost:5432/postgres", AppConfig.Database.DSN)
	require.Equal(t, []string{"http://localhost:3301", "http://127.0.0.1:3301"}, AppConfig.CORS.Origins)
}

func TestAllowDestructiveMigrationsEnvOverride(t *testing.T) {
	cases := []struct {
		name string
		env  string
		set  bool
		want bool
	}{
		{name: "enabled when value is 1", env: "1", set: true, want: true},
		{name: "disabled when unset", env: "", set: false, want: false},
		{name: "disabled when value is not 1", env: "true", set: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("MIGRATIONS_ALLOW_DESTRUCTIVE", tc.env)
			}
			viper.Reset()
			t.Cleanup(func() {
				viper.Reset()
				AppConfig = nil
			})

			require.NoError(t, LoadConfig("./definitely-missing"))
			require.NotNil(t, AppConfig)
			require.Equal(t, tc.want, AppConfig.Database.AllowDestructiveMigrations)
		})
	}
}
