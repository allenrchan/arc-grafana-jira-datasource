import {DataSourceJsonData, SelectableValue} from '@grafana/data';
import {DataQuery} from '@grafana/schema';

export interface JiraQuery extends DataQuery {
  jqlQuery: string;
  quantile: number;
  startStatus: string;
  endStatus: string;
  activeStatuses: string;
  metric: string;
}

export const METRICS = {
  CYCLE_TIME : 'cycletime',
  ISSUE_FLOW : 'issue_flow',
  NONE : 'none',
  CHANGELOG_RAW: 'changelogRaw',
  JQL: 'jql',
}

export const DEFAULT_QUERY: Partial<JiraQuery> = {
  metric: METRICS.NONE,
  quantile: 85,
};

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  url?: string;
  username?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  token?: string;
  basicAuth?: string;
}

export type QueryTypesResponse = {
  queryTypes: Array<SelectableValue<string>>;
};


export type StatusTypesResponse = {
  statusTypes: Array<SelectableValue<string>>;
};
