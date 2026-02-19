# ARC Grafana Jira Datasource

This is a high-performance Grafana datasource plugin that connects to **Jira Cloud** to visualize issue data, calculate **Cycle Time** metrics, and perform advanced **Flow Analysis** (Throughput, WIP, Lead Time, Efficiency).

It enables engineering teams to build powerful efficiency dashboards out-of-the-box without complex external ETL pipelines.

## Features

### 🚀 Flow Metrics (Issue Flow)
A comprehensive metric mode calculated in the backend for performance.
*   **Throughput**: Number of items completed within the selected time window (Daily/Weekly/Monthly).
*   **Cycle Time & Lead Time**: Precise calculation of time spent from "Start" to "End" statuses, or from Creation to Completion.
*   **Work In Progress (WIP)**: Active items that have started but not yet finished.
*   **Flow Efficiency**: Ratio of *Active Time* (value-add) vs *Total Cycle Time*, excluding waiting states.
*   **Age Analysis**: Track `AgeInCurrentStatus` for stale WIP tickets to spot bottlenecks.

### 🧠 Intelligent Data Handling
*   **Smart JQL Filtering**: Automatically fetches "Stale WIP" (old tickets currently In Progress) while filtering out thousands of irrelevant old "Done" or "Backlog" tickets.
*   **Window Clipping**: Backend logic filters out tickets completed *before* the dashboard time window to ensure Throughput charts don't show misleading historical data.
*   **Bulk Move Detection**: Intelligently detects tickets that were "Bulk Moved" to Done recently (which would skew metrics) and uses their *original* completion date instead.

### 🔌 Rich Field Support
Automatically fetches and exposes standard and custom fields for filtering/grouping:
*   **Standard**: Key, Summary, Status, Resolution, Status Category, Issue Type, Project, Created/Updated/Started/Completed Dates.
*   **Assignees**: Assignee, Engineer Assignee, QA Assignee (Names, Emails, IDs).
*   **Metrics**: Story Points, QA Story Points.

### 🔒 Secure
*   **Backend Proxy**: Uses a Go backend to proxy requests to Jira, keeping API tokens secure (never exposed to the browser).

---

## Configuration

1.  **Settings**:
    *   **URL**: Your Jira Cloud instance URL (e.g., `https://your-domain.atlassian.net`).
    *   **Email**: The email address of your Atlassian account.
    *   **API Token**: Create an API token at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens) and paste it here.
2.  **Save & Test**: Click "Save & Test" to verify the connection.

---

## Included Dashboards

This plugin includes a pre-built **Jira Flow Metrics** dashboard. You can import it from the Dashboards tab in the data source configuration page.
*   **Jira Flow Metrics**: A complete view of Cycle Time, Throughput, WIP, and Work Profile.

---

## Usage

### Query Editor

In a dashboard panel, select the datasource and configure the query:

*   **Metric**: Choose the type of data to visualize.
    *   **Flow Metrics**: The primary metric for Cycle Time, Throughput, and WIP dashboards.
    *   **JQL (Raw Issue Data)**: Simple table of issues for lists and drill-downs.
*   **JQL Query**: Enter your JQL query (e.g., `project IN ('PROJ', 'QA')`).
    *   *Tip*: The plugin automatically appends optimized time filters to fetch relevant data efficiently.
*   **Start Status**: Comma-separated list of statuses that mark the *start* of the cycle (e.g., `In Progress, Selected for Dev`).
*   **End Status**: Comma-separated list of statuses that mark the *completion* of the cycle (e.g., `Done, Resolved`).
    *   *Important*: Do **not** include "Won't Do" here if you want to exclude cancelled items from Cycle Time stats.
*   **Active Statuses**: Comma-separated list of statuses considered "Active" (vs. Waiting) for Flow Efficiency calculations.

### Dashboarding Recipes

#### 1. Cycle Time Scatter Plot
*   **Visualization**: Time Series (Points).
*   **Data Source**: Flow Data.
*   **X-Axis**: `Completed` (Time).
*   **Y-Axis**: `CycleTimeDays`.
*   **Threshold**: Use a Dashboard Variable to query the `Quantile` value (returned in the dataset) and map it to a Threshold line.
*   **Link**: Add a Data Link with URL `${__data.fields.IssueLink}` to open Jira.

#### 2. Throughput (Daily Bars)
*   **Visualization**: Bar Chart.
*   **X-Axis**: `CompletedDate` (String) - *Note: Using the string field ensures daily buckets align correctly without timezone shifts.*
*   **Y-Axis**: Count of `IssueKey`.
*   **Transform**: Group By `CompletedDate` -> Calculate Count.

#### 3. Work In Progress (WIP)
*   **Visualization**: Stat or Bar Gauge.
*   **Transform**: Filter by value -> `Completed` **Is Null**.
*   **Exclude Cancelled**: Add a filter `Resolution` is not `Won't Do` to remove cancelled items from your WIP board.

#### 4. Stale WIP / Age
*   **Visualization**: Table or Bar Gauge.
*   **Field**: `AgeInCurrentStatus`.
*   **Usage**: Visualize how long active items have been sitting in their current state to spot bottlenecks. Only populated for uncompleted items.
