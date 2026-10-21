package dashboards

import (
	"fmt"

	"github.com/grafana/grafana-foundation-sdk/go/dashboardv2"
	"github.com/grafana/grafana-foundation-sdk/go/prometheus"
	"github.com/grafana/grafana-foundation-sdk/go/timeseries"
)

func Build(routes []string) (dashboardv2.Dashboard, error) {
	builder := dashboardv2.NewDashboardBuilder("Highload HTTP").
		Editable(false).
		TimeSettings(
			dashboardv2.NewTimeSettingsBuilder().
				From("now-30m").
				To("now").
				AutoRefresh("5s")).
		Variables([]dashboardv2.VariableKind{
			labelValuesVariable("datacenter", "Datacenter",
				`label_values(http_request_duration_seconds_count, datacenter)`),
			labelValuesVariable("pod", "Pod",
				`label_values(http_request_duration_seconds_count{datacenter=~"$datacenter"}, pod)`),
		})

	layout := dashboardv2.NewAutoGridBuilder().
		MaxColumnCount(3)

	quantiles := []struct {
		value float64
		label string
	}{
		{0.50, "p50"},
		{0.75, "p75"},
		{0.90, "p90"},
		{0.95, "p95"},
		{0.99, "p99"},
	}

	for i, route := range routes {
		key := fmt.Sprintf("rps-%d", i+1)

		expression := fmt.Sprintf(
			`sum(rate(http_requests_total{route=%q,datacenter=~"$datacenter",pod=~"$pod"}[5m]))`,
			route,
		)

		query := prometheus.NewQueryV2Builder().
			Datasource(dashboardv2.NewDashboardv2DataQueryKindDatasourceBuilder().Name("victoriametrics")).
			Expr(expression).
			Range(true).
			Instant(false).
			LegendFormat("RPS")
		panel := dashboardv2.NewPanelBuilder().Id(float64(i + 1)).Title("RPS: " + route).Data(
			dashboardv2.NewQueryGroupBuilder().Target(
				dashboardv2.NewTargetBuilder().RefId("A").Query(query)),
		).Visualization(
			dashboardv2.NewVizConfigKindBuilder().
				Group("timeseries").
				Version("13.2.4").
				Options(timeseries.NewOptions()).
				Custom(timeseries.NewFieldConfig()).
				Unit("reqps").Min(0))

		builder.Panel(key, panel)
		layout.WithItem(key)

		errorsKey := fmt.Sprintf("errors-%d", i+1)
		errorsExpression := fmt.Sprintf(
			`sum by (status) (rate(http_requests_total{route=%q,datacenter=~"$datacenter",pod=~"$pod",status=~"4..|5.."}[5m]))`,
			route,
		)
		errorsQuery := prometheus.NewQueryV2Builder().
			Datasource(dashboardv2.NewDashboardv2DataQueryKindDatasourceBuilder().Name("victoriametrics")).
			Expr(errorsExpression).
			Range(true).
			Instant(false).
			LegendFormat("{{status}}")
		errorsPanel := dashboardv2.NewPanelBuilder().
			Id(float64(len(routes) + i + 1)).
			Title("Errors: " + route).
			Data(
				dashboardv2.NewQueryGroupBuilder().Target(
					dashboardv2.NewTargetBuilder().RefId("A").Query(errorsQuery)),
			).
			Visualization(
				dashboardv2.NewVizConfigKindBuilder().
					Group("timeseries").
					Version("13.2.4").
					Options(timeseries.NewOptions()).
					Custom(timeseries.NewFieldConfig()).
					Unit("reqps").Min(0),
			)

		builder.Panel(errorsKey, errorsPanel)
		layout.WithItem(errorsKey)

		latencyKey := fmt.Sprintf("latency-%d", i+1)
		latencyQueries := dashboardv2.NewQueryGroupBuilder()
		for j, quantile := range quantiles {
			latencyExpression := fmt.Sprintf(
				`histogram_quantile(%g, sum by (le) (rate(http_request_duration_seconds_bucket{route=%q,datacenter=~"$datacenter",pod=~"$pod"}[5m])))`,
				quantile.value,
				route,
			)
			latencyQuery := prometheus.NewQueryV2Builder().
				Datasource(dashboardv2.NewDashboardv2DataQueryKindDatasourceBuilder().Name("victoriametrics")).
				Expr(latencyExpression).
				Range(true).
				Instant(false).
				LegendFormat(quantile.label)
			latencyQueries.Target(
				dashboardv2.NewTargetBuilder().
					RefId(fmt.Sprintf("%c", 'A'+j)).
					Query(latencyQuery),
			)
		}
		latencyPanel := dashboardv2.NewPanelBuilder().
			Id(float64(2*len(routes) + i + 1)).
			Title("Latency: " + route).
			Data(latencyQueries).
			Visualization(
				dashboardv2.NewVizConfigKindBuilder().
					Group("timeseries").
					Version("13.2.4").
					Options(timeseries.NewOptions()).
					Custom(timeseries.NewFieldConfig()).
					Unit("s").Min(0),
			)

		builder.Panel(latencyKey, latencyPanel)
		layout.WithItem(latencyKey)
	}
	return builder.AutoGridLayout(layout).Build()
}
