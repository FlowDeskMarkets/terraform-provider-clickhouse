package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/FlowdeskMarkets/terraform-provider-clickhouse/pkg/common"
	"github.com/FlowdeskMarkets/terraform-provider-clickhouse/pkg/datasources"
	"github.com/FlowdeskMarkets/terraform-provider-clickhouse/pkg/resources"
	"github.com/FlowdeskMarkets/terraform-provider-clickhouse/pkg/sdk"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const minRetryDelay = 100 * time.Millisecond

func init() {
	// Set descriptions to support markdown syntax, this will be used in document generation
	// and the language server.
	schema.DescriptionKind = schema.StringMarkdown

	// Customize the content of descriptions when output. For example you can add defaults on
	// to the exported descriptions if present.
	// schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
	// 	desc := s.Description
	// 	if s.Default != nil {
	// 		desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
	// 	}
	// 	return strings.TrimSpace(desc)
	// }
}

func validateNonNegative(field string) schema.SchemaValidateDiagFunc {
	return func(i any, path cty.Path) diag.Diagnostics {
		value, ok := i.(int)
		if !ok {
			return diag.Diagnostics{diag.Diagnostic{
				Severity:      diag.Error,
				Summary:       fmt.Sprintf("%s must be an integer", field),
				AttributePath: path,
			}}
		}

		if value < 0 {
			return diag.Diagnostics{diag.Diagnostic{
				Severity:      diag.Error,
				Summary:       fmt.Sprintf("%s must be greater than or equal to 0", field),
				AttributePath: path,
			}}
		}

		return nil
	}
}

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		return &schema.Provider{
			Schema: map[string]*schema.Schema{
				"default_cluster": {
					Description: "Default cluster, if provided will be used when no cluster is provided",
					Type:        schema.TypeString,
					Optional:    true,
				},
				"username": {
					Description: "Clickhouse username with admin privileges",
					Type:        schema.TypeString,
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("TF_VAR_CLICKHOUSE_USERNAME", "default"),
				},
				"password": {
					Description: "Clickhouse user password with admin privileges",
					Type:        schema.TypeString,
					Optional:    true,
					Sensitive:   true,
					DefaultFunc: schema.EnvDefaultFunc("TF_VAR_CLICKHOUSE_PASSWORD", ""),
				},
				"host": {
					Description: "Clickhouse server URL",
					Type:        schema.TypeString,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("TF_VAR_CLICKHOUSE_HOST", "127.0.0.1"),
				},
				"port": {
					Description: "Clickhouse server native protocol port (TCP)",
					Type:        schema.TypeInt,
					Required:    true,
					DefaultFunc: schema.EnvDefaultFunc("TF_VAR_CLICKHOUSE_PORT", 9000),
				},
				"secure": {
					Description: "Clickhouse secure connection",
					Type:        schema.TypeBool,
					Optional:    true,
					Default:     false,
				},
				"dial_timeout": {
					Description:      "Timeout for establishing a connection to ClickHouse (in seconds). Useful for services that may need time to wake up.",
					Type:             schema.TypeInt,
					Optional:         true,
					Default:          30,
					ValidateDiagFunc: validateNonNegative("dial_timeout"),
				},
				"read_timeout": {
					Description:      "Timeout for reading data from ClickHouse (in seconds).",
					Type:             schema.TypeInt,
					Optional:         true,
					Default:          300,
					ValidateDiagFunc: validateNonNegative("read_timeout"),
				},
				"max_retries": {
					Description:      "Maximum number of retry attempts when connecting to ClickHouse. Set to 0 to disable retries.",
					Type:             schema.TypeInt,
					Optional:         true,
					Default:          0,
					ValidateDiagFunc: validateNonNegative("max_retries"),
				},
				"retry_delay": {
					Description:      "Initial delay between retry attempts (in seconds). The delay increases exponentially with each retry.",
					Type:             schema.TypeInt,
					Optional:         true,
					Default:          5,
					ValidateDiagFunc: validateNonNegative("retry_delay"),
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"clickhouse_dbs": datasources.DataSourceDbs(),
			},
			ResourcesMap: map[string]*schema.Resource{
				"clickhouse_db":    resources.ResourceDb(),
				"clickhouse_table": resources.ResourceTable(),
				"clickhouse_view":  resources.ResourceView(),
				"clickhouse_role":  resources.ResourceRole(),
				"clickhouse_user":  resources.ResourceUser(),
			},
			ConfigureContextFunc: configure(),
		}
	}
}

func configure() func(context.Context, *schema.ResourceData) (any, diag.Diagnostics) {
	return func(ctx context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
		host := d.Get("host").(string)
		port := d.Get("port").(int)
		username := d.Get("username").(string)
		password := d.Get("password").(string)
		secure := d.Get("secure").(bool)
		dialTimeout := time.Duration(d.Get("dial_timeout").(int)) * time.Second
		readTimeout := time.Duration(d.Get("read_timeout").(int)) * time.Second
		maxRetries := d.Get("max_retries").(int)
		retryDelay := time.Duration(d.Get("retry_delay").(int)) * time.Second

		var TLSConfig *tls.Config
		// To use TLS it's necessary to set the TLSConfig field as not nil
		if secure {
			TLSConfig = &tls.Config{
				InsecureSkipVerify: false,
			}
		}
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{fmt.Sprintf("%s:%d", host, port)},
			Auth: clickhouse.Auth{
				Username: username,
				Password: password,
			},
			Debug: common.DebugEnabled,
			Debugf: func(format string, v ...any) {
				if common.DebugEnabled {
					fmt.Printf(format, v...)
				}
			},
			Settings: clickhouse.Settings{
				"max_execution_time": 300,
			},
			DialTimeout: dialTimeout,
			ReadTimeout: readTimeout,
			TLS:         TLSConfig,
		})

		var diags diag.Diagnostics

		if err != nil {
			return nil, diag.FromErr(fmt.Errorf("error connecting to clickhouse: %v", err))
		}

		if err := pingWithRetry(ctx, conn, maxRetries, retryDelay); err != nil {
			return nil, diag.FromErr(fmt.Errorf("ping clickhouse database: %w", err))
		}

		return &sdk.Client{Conn: conn}, diags
	}
}

type pinger interface {
	Ping(context.Context) error
}

// pingWithRetry attempts to ping the ClickHouse connection with exponential backoff retry logic.
// This is useful for services that may take time to wake up from an idle state.
func pingWithRetry(ctx context.Context, conn pinger, maxRetries int, retryDelay time.Duration) error {
	var lastErr error
	delay := max(retryDelay, minRetryDelay)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := conn.Ping(ctx); err != nil {
			lastErr = err
			if attempt < maxRetries {
				jitter := time.Duration(rand.Int63n(int64(delay/2))) - delay/4
				sleep := delay + jitter

				timer := time.NewTimer(sleep)
				select {
				case <-ctx.Done():
					timer.Stop()
					return fmt.Errorf("context cancelled while retrying: %w", ctx.Err())
				case <-timer.C:
				}

				delay *= 2
			}
		} else {
			return nil
		}
	}

	if maxRetries > 0 {
		return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
	}
	return lastErr
}
