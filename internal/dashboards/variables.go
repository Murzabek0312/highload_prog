package dashboards

import "github.com/grafana/grafana-foundation-sdk/go/dashboardv2"

func labelValuesVariable(name, label, query string) dashboardv2.VariableKind {
	datasourceUID := "victoriametrics"
	allText := "All"
	allValue := ".*"

	variable := dashboardv2.NewQueryVariableKind()
	variable.Spec.Name = name
	variable.Spec.Label = &label
	variable.Spec.Definition = &query
	variable.Spec.Refresh = dashboardv2.VariableRefreshOnTimeRangeChanged
	variable.Spec.Sort = dashboardv2.VariableSortAlphabeticalAsc
	variable.Spec.Multi = true
	variable.Spec.IncludeAll = true
	variable.Spec.AllValue = &allValue
	variable.Spec.AllowCustomValue = false
	variable.Spec.Current = dashboardv2.VariableOption{
		Text: dashboardv2.StringOrArrayOfString{
			String: &allText,
		},
		Value: dashboardv2.StringOrArrayOfString{
			ArrayOfString: []string{"$__all"},
		},
	}
	variable.Spec.Query = dashboardv2.DataQueryKind{
		Kind:    "DataQuery",
		Group:   "prometheus",
		Version: "v0",
		Datasource: &dashboardv2.Dashboardv2DataQueryKindDatasource{
			Name: &datasourceUID,
		},
		Spec: map[string]any{
			// Grafana's Prometheus variable query type: label values.
			"qryType": 1,
			"query":   query,
			"refId":   "PrometheusVariableQueryEditor-VariableQuery",
		},
	}

	return dashboardv2.VariableKind{QueryVariableKind: variable}
}
