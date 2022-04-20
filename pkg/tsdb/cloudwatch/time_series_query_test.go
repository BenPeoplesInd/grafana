package cloudwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimeSeriesQuery(t *testing.T) {
	executor := newExecutor(nil, newTestConfig(), &fakeSessionCache{})
	now := time.Now()

	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})

	var cwClient fakeCWClient

	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}

	t.Run("Custom metrics", func(t *testing.T) {
		cwClient = fakeCWClient{
			CloudWatchAPI: nil,
			GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
				NextToken: nil,
				Messages:  []*cloudwatch.MessageData{},
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{
						StatusCode: aws.String("Complete"), Id: aws.String("a"), Label: aws.String("NetworkOut"), Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{&now},
					},
					{
						StatusCode: aws.String("Complete"), Id: aws.String("b"), Label: aws.String("NetworkIn"), Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{&now},
					},
				},
			},
		}

		im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
			return datasourceInfo{}, nil
		})

		executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})
		resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{
					RefID: "A",
					TimeRange: backend.TimeRange{
						From: now.Add(time.Hour * -2),
						To:   now.Add(time.Hour * -1),
					},
					JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"subtype":   "metrics",
						"namespace": "AWS/EC2",
						"metricName": "NetworkOut",
						"expression": "",
						"dimensions": {
						  "InstanceId": "i-00645d91ed77d87ac"
						},
						"region": "us-east-2",
						"id": "a",
						"alias": "NetworkOut",
						"statistics": [
						  "Maximum"
						],
						"period": "300",
						"hide": false,
						"matchExact": true,
						"refId": "A"
					}`),
				},
				{
					RefID: "B",
					TimeRange: backend.TimeRange{
						From: now.Add(time.Hour * -2),
						To:   now.Add(time.Hour * -1),
					},
					JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"subtype":   "metrics",
						"namespace": "AWS/EC2",
						"metricName": "NetworkIn",
						"expression": "",
						"dimensions": {
						"InstanceId": "i-00645d91ed77d87ac"
						},
						"region": "us-east-2",
						"id": "b",
						"alias": "NetworkIn",
						"statistics": [
						"Maximum"
						],
						"period": "300",
						"matchExact": true,
						"refId": "B"
					}`),
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "NetworkOut", resp.Responses["A"].Frames[0].Name)
		assert.Equal(t, "NetworkIn", resp.Responses["B"].Frames[0].Name)
	})

	t.Run("End time before start time should result in error", func(t *testing.T) {
		_, err := executor.executeTimeSeriesQuery(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{{TimeRange: backend.TimeRange{
			From: now.Add(time.Hour * -1),
			To:   now.Add(time.Hour * -2),
		}}}})
		assert.EqualError(t, err, "invalid time range: start time must be before end time")
	})

	t.Run("End time equals start time should result in error", func(t *testing.T) {
		_, err := executor.executeTimeSeriesQuery(context.Background(), &backend.QueryDataRequest{Queries: []backend.DataQuery{{TimeRange: backend.TimeRange{
			From: now.Add(time.Hour * -1),
			To:   now.Add(time.Hour * -1),
		}}}})
		assert.EqualError(t, err, "invalid time range: start time must be before end time")
	})
}

func Test_QueryData_executeTimeSeriesQuery_alias_provided_frame_name_uses_period_and_stat_from_expression_when_isUserDefinedSearchExpression(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}

	cwClient = fakeCWClient{
		GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{StatusCode: aws.String("Complete"), Id: aws.String("a"), Label: aws.String("NetworkOut"),
					Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
			},
		},
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
				JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 1,
						"namespace": "",
						"metricName": "",
						"expression": "SEARCH('{AWS/EC2,InstanceId} MetricName=\"CPUUtilization\"', 'Average', 300)",
						"region": "us-east-2",
						"id": "a",
						"alias": "{{period}} {{stat}}",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": true,
						"refId": "A"
					}`),
			},
		},
	})

	assert.NoError(t, err)
	// asserts that period '300' and stat 'Average' are parsed from the JSON query expression and used in the alias, not the JSON model's 'statistic' nor 'period' fields
	assert.Equal(t, "300 Average", resp.Responses["A"].Frames[0].Name)
}

func Test_QueryData_executeTimeSeriesQuery_no_alias_provided_frame_name_is_queryId_when_query_isMathExpression(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}
	cwClient = fakeCWClient{
		GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String("NetworkOut"),
					Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
			},
		},
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
				JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 1,
						"namespace": "",
						"metricName": "",
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": true,
						"refId": "A"
					}`),
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "query id", resp.Responses["A"].Frames[0].Name)
}

func Test_QueryData_executeTimeSeriesQuery_no_alias_provided_frame_name_depends_on_dimension_values_and_matchExact(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}
	cwClient = fakeCWClient{
		GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String("response label"),
					Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
			},
		},
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	t.Run("frame name is label when isInferredSearchExpression and not isMultiValuedDimensionExpression", func(t *testing.T) {
		testCasesReturningLabel := map[string]struct {
			dimensions string
			matchExact bool
		}{
			"with specific dimension, matchExact false": {dimensions: `"dimensions": {"InstanceId": ["some-instance"]},`, matchExact: false},
			"with wildcard dimension, matchExact false": {dimensions: `"dimensions": {"InstanceId": ["*"]},`, matchExact: false},
			"with wildcard dimension, matchExact true":  {dimensions: `"dimensions": {"InstanceId": ["*"]},`, matchExact: true},
			"without dimension, matchExact false":       {dimensions: "", matchExact: false},
		}
		for name, tc := range testCasesReturningLabel {
			t.Run(name, func(t *testing.T) {
				resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
					PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
					Queries: []backend.DataQuery{
						{
							RefID:     "A",
							TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
							JSON: json.RawMessage(fmt.Sprintf(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 0,
						"namespace": "",
						"metricName": "",
						%s
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": %t,
						"refId": "A"
					}`, tc.dimensions, tc.matchExact)),
						},
					},
				})

				assert.NoError(t, err)
				assert.Equal(t, "response label", resp.Responses["A"].Frames[0].Name)
			})
		}
	})

	t.Run("frame name is metricName_stat when isInferredSearchExpression and not isMultiValuedDimensionExpression", func(t *testing.T) {
		testCasesReturningMetricStat := map[string]struct {
			dimensions string
			matchExact bool
		}{
			"with specific dimension, matchExact true": {dimensions: `"dimensions": {"InstanceId": ["some-instance"]},`, matchExact: true},
			"without dimension, matchExact true":       {dimensions: "", matchExact: true},
			"multi dimension, matchExact true":         {dimensions: `"dimensions": {"InstanceId": ["some-instance","another-instance"]},`, matchExact: true},
			"multi dimension, matchExact false":        {dimensions: `"dimensions": {"InstanceId": ["some-instance","another-instance"]},`, matchExact: false},
		}
		for name, tc := range testCasesReturningMetricStat {
			t.Run(name, func(t *testing.T) {
				resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
					PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
					Queries: []backend.DataQuery{
						{
							RefID:     "A",
							TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
							JSON: json.RawMessage(fmt.Sprintf(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 0,
						"metricEditorMode": 0,
						"namespace": "",
						"metricName": "CPUUtilization",
						%s
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": %t,
						"refId": "A"
					}`, tc.dimensions, tc.matchExact)),
						},
					},
				})

				assert.NoError(t, err)
				assert.Equal(t, "CPUUtilization_Maximum", resp.Responses["A"].Frames[0].Name)
			})
		}
	})
}

func Test_QueryData_executeTimeSeriesQuery_no_alias_provided_frame_name_is_label_when_query_type_is_MetricQueryTypeQuery(t *testing.T) {
	origNewCWClient := NewCWClient
	t.Cleanup(func() {
		NewCWClient = origNewCWClient
	})
	var cwClient fakeCWClient
	NewCWClient = func(sess *session.Session) cloudwatchiface.CloudWatchAPI {
		return &cwClient
	}

	cwClient = fakeCWClient{
		GetMetricDataOutput: cloudwatch.GetMetricDataOutput{
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{StatusCode: aws.String("Complete"), Id: aws.String("query id"), Label: aws.String("response label"),
					Values: []*float64{aws.Float64(1.0)}, Timestamps: []*time.Time{{}}},
			},
		},
	}
	im := datasource.NewInstanceManager(func(s backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
		return datasourceInfo{}, nil
	})
	executor := newExecutor(im, newTestConfig(), &fakeSessionCache{})

	resp, err := executor.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{}},
		Queries: []backend.DataQuery{
			{
				RefID:     "A",
				TimeRange: backend.TimeRange{From: time.Now().Add(time.Hour * -2), To: time.Now().Add(time.Hour * -1)},
				JSON: json.RawMessage(`{
						"type":      "timeSeriesQuery",
						"metricQueryType": 1,
						"metricEditorMode": 0,
						"namespace": "",
						"metricName": "",
						"dimensions": {"InstanceId": ["some-instance"]},
						"region": "us-east-2",
						"id": "query id",
						"statistic": "Maximum",
						"period": "1200",
						"hide": false,
						"matchExact": false,
						"refId": "A"
					}`),
			},
		},
	})

	assert.NoError(t, err)
	assert.Equal(t, "response label", resp.Responses["A"].Frames[0].Name)
}
