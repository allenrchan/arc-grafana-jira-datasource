import React, {ChangeEvent} from 'react';
import {InlineField, Input, Select} from '@grafana/ui';
import {QueryEditorProps, SelectableValue} from '@grafana/data';
import {DataSource} from '../../datasource';
import {MyDataSourceOptions, JiraQuery, METRICS} from '../../types';
import {useMetricTypes} from "./useQueryTypes";

type Props = QueryEditorProps<DataSource, JiraQuery, MyDataSourceOptions>;
type StatusSelectProps = {datasource: DataSource, query: JiraQuery, onChange: (value: JiraQuery) => void}

export function StartStatusSelect({datasource, query, onChange}: StatusSelectProps) {
    const onStatusChange = (event: ChangeEvent<HTMLInputElement>) => {
        onChange({...query, startStatus: event.target.value});
    };

    return (
        <InlineField label={'Start Status'} required={true}>
            <Input onChange={onStatusChange} value={query.startStatus} placeholder="e.g. In Progress, Review" />
        </InlineField>
    )
}

export function EndStatusSelect({datasource, query, onChange}: StatusSelectProps) {
    const onStatusChange = (event: ChangeEvent<HTMLInputElement>) => {
        onChange({...query, endStatus: event.target.value});
    };

    return (
        <InlineField label={'End Status'} required={true}>
            <Input onChange={onStatusChange} value={query.endStatus} placeholder="e.g. Done, Closed" />
        </InlineField>
    )
}

export function ActiveStatusSelect({datasource, query, onChange}: StatusSelectProps) {
    const onStatusChange = (event: ChangeEvent<HTMLInputElement>) => {
        onChange({...query, activeStatuses: event.target.value});
    };

    return (
        <InlineField label={'Active Statuses'} tooltip="Comma separated statuses to consider 'Active' for Flow Efficiency (e.g. In Progress, Review)">
            <Input onChange={onStatusChange} value={query.activeStatuses} placeholder="e.g. In Progress, Review" />
        </InlineField>
    )
}

export function QueryEditor({datasource, query, onChange, onRunQuery}: Props) {

    const { loading, queryTypes, error } = useMetricTypes(datasource);

    const onQueryTextChange = (event: ChangeEvent<HTMLInputElement>) => {
        onChange({...query, jqlQuery: event.target.value});
    };

    const onQuantileChange = (event: ChangeEvent<HTMLInputElement>) => {
        onChange({...query, quantile: parseFloat(event.target.value)});
    };

    const onMetricChange = (value: SelectableValue) => {
        onChange({...query, metric: value.value});
    };

    const {jqlQuery, quantile, metric} = query;


    return (
        <div className="gf-form">
            <InlineField label="JQl Query" labelWidth={16} htmlFor="query-jql" tooltip="Which JQL should be used? for example: project = 'FOOBAR' " invalid={!jqlQuery} error={"this field is required"} required={true}>
                <Input id="query-jql" onChange={onQueryTextChange} placeholder={'insert the JQL Query here'} value={jqlQuery}  />
            </InlineField>
            <InlineField label="Metric" htmlFor="query-metric" tooltip="Which metric you want to see? " invalid={!metric} error={"this field is required"} required={true}>
                <Select inputId="query-metric" onChange={onMetricChange} value={metric} options={queryTypes} isLoading={loading} disabled={!!error} />
            </InlineField>
            {(metric === METRICS.CYCLE_TIME || metric === METRICS.ISSUE_FLOW)
                ? <StartStatusSelect datasource={datasource}  onChange={onChange} query={query} ></StartStatusSelect>
                : ''
            }
            {(metric === METRICS.CYCLE_TIME || metric === METRICS.ISSUE_FLOW)
                ? <EndStatusSelect datasource={datasource}  onChange={onChange}  query={query} ></EndStatusSelect>
                : ''
            }
            {metric === METRICS.ISSUE_FLOW
                ? <ActiveStatusSelect datasource={datasource} onChange={onChange} query={query} />
                : ''
            }
            {metric === METRICS.CYCLE_TIME &&
            <InlineField label="Quantile" htmlFor="query-quantile" required={true}>
                <Input id="query-quantile" onChange={onQuantileChange} width={8} type="number" min={1} max={100} value={quantile}/>
            </InlineField>
            }
        </div>
    );
}
