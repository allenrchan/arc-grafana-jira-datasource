import {
    DataSourceInstanceSettings,
    ScopedVars,
} from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { JiraQuery, MyDataSourceOptions, DEFAULT_QUERY, METRICS, QueryTypesResponse } from './types';

export class DataSource extends DataSourceWithBackend<JiraQuery, MyDataSourceOptions> {
    constructor(instanceSettings: DataSourceInstanceSettings<MyDataSourceOptions>) {
        super(instanceSettings);
    }

    getDefaultQuery(_: any): Partial<JiraQuery> {
        return DEFAULT_QUERY;
    }

    applyTemplateVariables(query: JiraQuery, scopedVars: ScopedVars): JiraQuery {
        return {
            ...query,
            jqlQuery: getTemplateSrv().replace(query.jqlQuery, scopedVars),
            startStatus: getTemplateSrv().replace(query.startStatus, scopedVars),
            endStatus: getTemplateSrv().replace(query.endStatus, scopedVars),
            activeStatuses: getTemplateSrv().replace(query.activeStatuses, scopedVars),
        };
    }

    getAvailableMetricTypes(): Promise<QueryTypesResponse> {
        const metrics = [
            {value: METRICS.ISSUE_FLOW, label: 'Flow Metrics'},
            {value: METRICS.CYCLE_TIME, label: 'Cycle Time (Legacy)'},
            {value: METRICS.CHANGELOG_RAW, label: 'change log - raw data'},
            {value: METRICS.JQL, label: 'JQL (Raw Issue Data)'},
            {value: METRICS.NONE, label: 'None'},
        ]

        return Promise.resolve({queryTypes: metrics});
    }
}
