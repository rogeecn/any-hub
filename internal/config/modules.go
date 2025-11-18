package config

import (
	_ "github.com/any-hub/any-hub/internal/hubmodule/composer"
	_ "github.com/any-hub/any-hub/internal/hubmodule/debian"
	_ "github.com/any-hub/any-hub/internal/hubmodule/docker"
	_ "github.com/any-hub/any-hub/internal/hubmodule/golang"
	_ "github.com/any-hub/any-hub/internal/hubmodule/legacy"
	_ "github.com/any-hub/any-hub/internal/hubmodule/apk"
	_ "github.com/any-hub/any-hub/internal/hubmodule/npm"
	_ "github.com/any-hub/any-hub/internal/hubmodule/pypi"
)
