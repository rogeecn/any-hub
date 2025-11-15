package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/any-hub/any-hub/internal/cache"
	"github.com/any-hub/any-hub/internal/config"
	"github.com/any-hub/any-hub/internal/hubmodule"
	"github.com/any-hub/any-hub/internal/logging"
	"github.com/any-hub/any-hub/internal/proxy"
	"github.com/any-hub/any-hub/internal/server"
	"github.com/any-hub/any-hub/internal/server/routes"
	"github.com/any-hub/any-hub/internal/version"
)

// cliOptions 汇总 CLI 标志解析后的结果，便于在测试中注入。
type cliOptions struct {
	configPath  string
	checkOnly   bool
	showVersion bool
}

var (
	stdOut io.Writer = os.Stdout
	stdErr io.Writer = os.Stderr
)

func main() {
	opts, err := parseCLIFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(stdErr, err.Error())
		os.Exit(2)
	}
	os.Exit(run(opts))
}

// run 根据解析到的 CLI 选项执行业务流程，并返回退出码，方便测试。
func run(opts cliOptions) int {
	if opts.showVersion {
		printVersion()
		return 0
	}

	cfg, err := config.Load(opts.configPath)
	if err != nil {
		fmt.Fprintf(stdErr, "加载配置失败: %v\n", err)
		return 1
	}

	logger, err := logging.InitLogger(cfg.Global)
	if err != nil {
		fmt.Fprintf(stdErr, "初始化日志失败: %v\n", err)
		return 1
	}

	if opts.checkOnly {
		fields := logging.BaseFields("check_config", opts.configPath)
		fields["hubs"] = len(cfg.Hubs)
		fields["credentials"] = config.CredentialModes(cfg.Hubs)
		fields["result"] = "ok"
		logger.WithFields(fields).Info("配置校验通过")
		return 0
	}

	registry, err := server.NewHubRegistry(cfg)
	if err != nil {
		fmt.Fprintf(stdErr, "构建 Hub 注册表失败: %v\n", err)
		return 1
	}

	// CLI 启动遵循“配置 → HubRegistry → 磁盘缓存 → Fiber server”顺序，
	// 保证所有请求共享统一的路由与缓存实例，方便观察 cache/log 指标。
	store, err := cache.NewStore(cfg.Global.StoragePath)
	if err != nil {
		fmt.Fprintf(stdErr, "初始化缓存目录失败: %v\n", err)
		return 1
	}

	httpClient := server.NewUpstreamClient(cfg)
	proxyHandler := proxy.NewHandler(httpClient, logger, store)
	forwarder := proxy.NewForwarder(proxyHandler)
	proxy.RegisterModuleHandler(hubmodule.DefaultModuleKey(), proxyHandler)

	fields := logging.BaseFields("startup", opts.configPath)
	fields["hubs"] = len(cfg.Hubs)
	fields["listen_port"] = cfg.Global.ListenPort
	fields["credentials"] = config.CredentialModes(cfg.Hubs)
	fields["version"] = version.Full()
	logger.WithFields(fields).Info("配置加载完成")

	if err := startHTTPServer(cfg, registry, forwarder, logger); err != nil {
		fmt.Fprintf(stdErr, "HTTP 服务启动失败: %v\n", err)
		return 1
	}
	return 0
}

// parseCLIFlags 解析 CLI 参数，并结合环境变量计算最终的配置路径。
func parseCLIFlags(args []string) (cliOptions, error) {
	fs := flag.NewFlagSet("any-hub", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		configFlag string
		checkOnly  bool
		showVer    bool
	)

	fs.StringVar(&configFlag, "config", "", "配置文件路径（默认 ./config.toml，可被 ANY_HUB_CONFIG 覆盖）")
	fs.BoolVar(&checkOnly, "check-config", false, "仅校验配置后退出")
	fs.BoolVar(&showVer, "version", false, "显示版本信息")

	if err := fs.Parse(args); err != nil {
		return cliOptions{}, fmt.Errorf("解析参数失败: %w", err)
	}

	path := os.Getenv("ANY_HUB_CONFIG")
	if configFlag != "" {
		path = configFlag
	}
	if path == "" {
		path = "config.toml"
	}

	return cliOptions{
		configPath:  path,
		checkOnly:   checkOnly,
		showVersion: showVer,
	}, nil
}

func startHTTPServer(cfg *config.Config, registry *server.HubRegistry, proxyHandler server.ProxyHandler, logger *logrus.Logger) error {
	port := cfg.Global.ListenPort
	app, err := server.NewApp(server.AppOptions{
		Logger:     logger,
		Registry:   registry,
		Proxy:      proxyHandler,
		ListenPort: port,
	})
	if err != nil {
		return err
	}
	routes.RegisterModuleRoutes(app, registry)

	logger.WithFields(logrus.Fields{
		"action": "listen",
		"port":   port,
	}).Info("Fiber 服务启动")

	return app.Listen(fmt.Sprintf(":%d", port))
}
