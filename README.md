# Arc Grafana Jira Datasource

This is a Grafana datasource plugin that connects to **Jira Cloud** to visualize issue data, calculate **Cycle Time** metrics, and perform advanced **Flow Analysis** (Throughput, WIP, Lead Time, Efficiency).

It is designed to be a high-performance, backend-driven datasource that handles the complexity of Jira's changelog history, custom fields, and large datasets, enabling you to build engineering efficiency dashboards out-of-the-box.

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

### 🔒 Secure & Optimized
*   **Backend Proxy**: Uses a Go backend to proxy requests to Jira, keeping API tokens secure (never exposed to the browser).
*   **Pagination**: Handles Jira API pagination automatically.
*   **Caching**: (Future) Potential for caching heavy changelog responses.

---

## Configuration

1.  **Add Data Source**: In Grafana, go to **Configuration** > **Data Sources** > **Add data source**, and select "Arc Grafana Jira Datasource".
2.  **Settings**:
    *   **URL**: Your Jira Cloud instance URL (e.g., `https://your-domain.atlassian.net`).
    *   **Email**: The email address of your Atlassian account.
    *   **API Token**: Create an API token at [id.atlassian.com](https://id.atlassian.com/manage-profile/security/api-tokens) and paste it here.
3.  **Save & Test**: Click "Save & Test" to verify the connection.

---

## Usage

### Query Editor

In a dashboard panel, select the datasource and configure the query:

*   **Metric**: Choose the type of data to visualize.
    *   **Flow Metrics**: The primary metric for Cycle Time, Throughput, and WIP dashboards.
    *   **JQL (Raw Issue Data)**: Simple table of issues for lists and drill-downs.
*   **JQL Query**: Enter your JQL query (e.g., `project IN ('PROJ', 'QA')`).
    *   *Tip*: The plugin appends `AND (updated >= ... OR statusCategory = In Progress)` automatically. You do not need to manually manage time windows.
*   **Start Status**: Comma-separated list of statuses that mark the *start* of the cycle (e.g., `In Progress, Selected for Dev`).
*   **End Status**: Comma-separated list of statuses that mark the *completion* of the cycle (e.g., `Done, Resolved`).
    *   *Important*: Do **not** include "Won't Do" or "Duplicate" here if you want to exclude cancelled items from Cycle Time stats.
*   **Active Statuses**: Comma-separated list of statuses considered "Active" (vs. Waiting) for Flow Efficiency calculations.

### Dashboarding Recipes

#### 1. Cycle Time Scatter Plot
*   **Visualization**: Time Series (Points).
*   **Data Source**: Flow Data.
*   **X-Axis**: `Completed` (Time).
*   **Y-Axis**: `CycleTimeDays`.
*   **Threshold**: Use a Dashboard Variable to query the `Quantile` value (returned in the dataset) and map it to a Threshold line.
*   **Link**: Add a Data Link with URL `${__data.fields.IssueLink}` (or construct using IssueKey) to open Jira.

#### 2. Throughput (Daily Bars)
*   **Visualization**: Bar Chart.
*   **X-Axis**: `CompletedDate` (String) - *Note: Using the string field ensures daily buckets align correctly without timezone shifts.*
*   **Y-Axis**: Count of `IssueKey`.
*   **Transform**: Group By `CompletedDate` -> Calculate Count.
*   **Stacking**: Group by `Project` or `IssueType` to stack bars.

#### 3. Work In Progress (WIP)
*   **Visualization**: Stat or Bar Gauge.
*   **Transform**: Filter by value -> `Completed` **Is Null**.
*   **Exclude Cancelled**: Add a filter `Resolution` is not `Won't Do` to remove cancelled items from your WIP board.

#### 4. Stale WIP / Age
*   **Visualization**: Table or Bar Gauge.
*   **Field**: `AgeInCurrentStatus`.
*   **Usage**: Visualize how long active items have been sitting in their current state to spot bottlenecks. Only populated for uncompleted items.

---

## Development

### Prerequisites

*   Node.js (v22+)
*   pnpm
*   Go (v1.21+)
*   Mage

### Backend
1.  Update Grafana plugin SDK for Go dependency to the latest minor version:
    ```bash
    go get -u github.com/grafana/grafana-plugin-sdk-go
    go mod tidy
    ```
2.  Build plugin backend binaries for Linux, Windows and Darwin:
    ```bash
    mage -v
    ```
3.  List all available Mage targets for additional commands:
    ```bash
    mage -l
    ```

### Frontend
1.  Install dependencies
    ```bash
    pnpm install
    ```
2.  Build plugin in development mode and run in watch mode
    ```bash
    pnpm run dev
    ```
3.  Build plugin in production mode
    ```bash
    pnpm run build
    ```
4.  Run the tests (using Jest)
    ```bash
    pnpm run test
    pnpm run test:ci
    ```
5.  Spin up a Grafana instance and run the plugin inside it (using Docker)
    ```bash
    pnpm run server
    ```
6.  Run the linter
    ```bash
    pnpm run lint
    ```

## Distributing your plugin

When distributing a Grafana plugin either within the community or privately the plugin must be signed so the Grafana application can verify its authenticity. This can be done with the `@grafana/sign-plugin` package.

### Signing a plugin

1.  Create a [Grafana Cloud account](https://grafana.com/signup).
2.  Create a Grafana Cloud Access Policy Token with the `PluginPublisher` role.
3.  Sign the plugin:
    ```bash
    export GRAFANA_ACCESS_POLICY_TOKEN=<your-token>
    pnpm dlx @grafana/sign-plugin@latest --rootUrls https://your-grafana-instance.com
    ```
To create a Scatter Plot with different colors per project:
1.  Use a JQL that selects multiple projects.
2.  In the Panel Editor, go to **Transformations**.
3.  Add **"Partition by values"**.
4.  Select the **"Project"** field.
5.  This will split the data into separate series for each project.

### Template Variables

You can use dashboard variables in your query fields to make dashboards interactive:
*   **JQL**: `project IN (${project:singlequote})`
*   **Status**: `${StartStatus}` (Mult-value variables are supported)

## Development

### Prerequisites

*   Node.js (v22+)
*   pnpm
*   Go (v1.21+)
*   Mage

### Build

1.  **Frontend**:
    ```bash
    pnpm install
    pnpm run build
    ```
2.  **Backend**:
    ```bash
    # Build for current platform
    mage -v build:backend
    
    # Build for Linux (e.g. for Docker/Production)
    mage -v build:linux
    ```

### Run Locally

```bash
pnpm run server
```
This starts a local Grafana instance with the plugin installed.
