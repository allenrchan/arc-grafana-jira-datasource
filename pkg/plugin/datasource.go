package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/allenrchan/arc-grafana-jira-datasource/pkg/jira"
	"github.com/allenrchan/arc-grafana-jira-datasource/pkg/models"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin in
// runtime. In this example datasource instance implements backend.QueryDataHandler,
// backend.CheckHealthHandler interfaces. Plugin should not implement all these
// interfaces - only those which are required for a particular task.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// NewDatasource creates a new datasource instance.
func NewDatasource(_ context.Context, _ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	return &Datasource{}, nil
}

// Datasource is an example datasource which can respond to data queries, reports
// its health and has streaming skills.
type Datasource struct{}

// Dispose here tells plugin SDK that plugin wants to clean up resources when a new instance
// created. As soon as datasource settings change detected by SDK old datasource instance will
// be disposed and a new one will be created using NewSampleDatasource factory function.
func (d *Datasource) Dispose() {
	// Clean up datasource instance resources.
}

// QueryData handles multiple queries and returns multiple responses.
// req contains the queries []DataQuery (where each query contains RefID as a unique identifier).
// The QueryDataResponse contains a map of RefID to the response for each query, and each response
// contains Frames ([]*Frame).
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	// create response struct
	response := backend.NewQueryDataResponse()

	config, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	client := jira.NewClient(config.URL, config.Username, config.Secrets.Token)

	// loop over queries and execute them individually.
	for _, q := range req.Queries {
		res := d.query(ctx, client, q, config.URL)

		// save the response in a hashmap
		// based on with RefID as identifier
		response.Responses[q.RefID] = res
	}

	return response, nil
}

type queryModel struct {
	JQLQuery    string  `json:"jqlQuery"`
	Quantile       float64 `json:"quantile"`
	StartStatus    string  `json:"startStatus"`
	EndStatus      string  `json:"endStatus"`
	ActiveStatuses string  `json:"activeStatuses"`
	Metric         string  `json:"metric"`
}

func (d *Datasource) query(_ context.Context, client *jira.Client, query backend.DataQuery, baseURL string) backend.DataResponse {
	// var response backend.DataResponse // Unused variable removed

	// Unmarshal the JSON into our queryModel.
	var qm queryModel

	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("json unmarshal: %v", err.Error()))
	}

	// Append time range filter to JQL to reduce load
	// Format: "YYYY-MM-DD HH:mm"
	// Example: "project = PLAT AND updated >= '2023-01-01 00:00'"
	// We only care about From time because filtering "To" might exclude issues updated *after* the window but were active *during* the window?
	// Actually, if we want cycle time in a window, the issue must have had activity. 
	// "updated >= From" is safe because if it wasn't updated since From, it couldn't have transitioned in that window (except if we care about "open during", but cycle time is about transitions).
	// For "open tickets" count, we might need different logic, but for cycle time (transitions) and changelog history, "updated >= From" is correct.
	// For JQL metric (raw issues), if we want "issues active in window", "updated >= From" is also a good proxy or "updated >= From OR created >= From".
	// But "updated >= From" covers created too (creation is an update).
	// One edge case: Issue created before From, Updated before From, but still Open. It won't be fetched. 
	// But if it wasn't updated in the window, it didn't change status in the window, so cycle time/changelog won't have entries in the window anyway.
	// So "updated >= From" is safe optimization.
	
	jql := qm.JQLQuery
	if jql != "" {
		fromTime := query.TimeRange.From.Format("2006-01-02 15:04")
		// Check if JQL already has "order by" to avoid syntax error (order by must be last)
		// Basic check: split by "order by" (case insensitive)
		// This is tricky parsing. For now, let's append it at the END if no order by, or insert it.
		// A safer way: Use parentheses? "(original_jql) AND updated >= ..."
		// But "order by" must be outside parens.
		// If user provides "order by", we might break it. 
		// Simpler approach: Assume user might provide "order by".
		// We can try to append it. If JQL has "order by", we should insert before it.
		// But regex is fragile.
		// Alternative: Just append it and warn user? No.
		// Let's just append " AND updated >= ..." and hope user puts "order by" at end if at all? 
		// Actually, standard JQL allows "AND" clauses before "ORDER BY". 
		// If the user's string ends with "ORDER BY ...", appending " AND ..." is invalid syntax.
		// 
		// Let's keep it simple: Append it. If the user has "ORDER BY", they should move it or we accept it might fail for complex custom JQLs.
		// Or... we can check if "order by" exists and insert before it.
		// But wait, the previous `SearchChangelogs` logic paginates. It doesn't rely on sorting?
		// Actually, `jql` is passed as string.
		// Let's implement a simple heuristic: if "order by" is found, insert before it.
		
		// Note: We won't implement complex parsing now. Just append it, assuming most users put filter logic first.
		// If "ORDER BY" is present, we wrap the original query in parens? No, parens don't work around ORDER BY.
		// Let's just assume valid JQL structure.
		// Ideally we would put: `(user_jql) AND updated >= '...'`.
		// But if user_jql has ORDER BY, `( ... ORDER BY ...) AND ...` is invalid.
		
		// Decision: Append ` AND updated >= '...'`.
		// Limitation: User JQL must NOT end with ORDER BY for this to work perfectly.
		// Or we can warn.
		
		// Updated Logic: We want to fetch tickets that are either:
		// 1. Currently IN PROGRESS (statusCategory = 4, "In Progress" / "Blue" / "Yellow")
		//    This catches "Stale WIP" - items started long ago but sitting untouched.
		// 2. BACKLOG items (statusCategory = 2, "To Do" / "Grey") ONLY if updated recently.
		//    This avoids fetching the entire historic backlog.
		// 3. DONE items (statusCategory = 3, "Done" / "Green") ONLY if they BECAME Done recently.
		//    We use statusCategoryChangedDate >= From to filter out tickets that were bulk-edited (updated=recent)
		//    but whose "Done" state hasn't actually changed (statusCategoryChangedDate=old).
		
		jql += fmt.Sprintf(" AND (statusCategory = 4 OR (statusCategory = 2 AND updated >= '%s') OR (statusCategory = 3 AND statusCategoryChangedDate >= '%s'))", fromTime, fromTime)
	}

	// Fetch issues from Jira
	issues, err := client.SearchChangelogs(jql)
	if err != nil {
		// backend.StatusInternalServerError is not exported or valid in this SDK version likely.
		// Using backend.StatusBadRequest or constructing error with status.
		// Standard way is to return DataResponse with Error field.
		return backend.ErrDataResponse(backend.StatusInternal, fmt.Sprintf("jira search failed: %v", err.Error()))
	}

	switch qm.Metric {
	case "changelogRaw":
		return d.getChangelogRawData(issues)
	case "jql":
		return d.getJQLData(issues)
	case "issue_flow":
		return d.getFlowData(issues, qm, query.TimeRange, baseURL)
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unknown metric: %s", qm.Metric))
	}
}

// Struct to hold analyzed issue data
type issueData struct {
	Issue           jira.Issue
	Created         time.Time
	Started         *time.Time
	Completed       *time.Time
	LeadTimeDays    *float64
	CycleTimeDays   *float64
	ActiveTimeDays  *float64
	FlowEfficiency  *float64
	Assignee        string
	AssigneeEmail   string
	AssigneeID      string
	Engineer        string
	EngineerEmail   string
	EngineerID      string
	QA              string
	QAEmail         string
	QAID            string
	StoryPoints     *float64
	QAStoryPoints   *float64
	Status          string
	StatusCategory  string
	Resolution      string
	AgeInCurrentStatus *float64
}

// Helper to analyze a single issue
func (d *Datasource) analyzeIssue(issue jira.Issue, startStatuses, endStatuses, activeStatuses map[string]bool) issueData {
	data := issueData{Issue: issue}

		// Parse Created Time
		createdStr, _ := issue.Fields["created"].(string)
		if t, err := time.Parse("2006-01-02T15:04:05.000-0700", createdStr); err == nil {
			data.Created = t
		}

		// Helper to extract user fields (Display Name and Email)
		getUser := func(field string) (string, string, string) {
			if u, ok := issue.Fields[field].(map[string]interface{}); ok {
				displayName, _ := u["displayName"].(string)
				emailAddress, _ := u["emailAddress"].(string)
				accountId, _ := u["accountId"].(string)
				
				if displayName == "" {
					displayName = "Unassigned"
				}
				return displayName, emailAddress, accountId
			}
			return "Unassigned", "", ""
		}

		data.Assignee, data.AssigneeEmail, data.AssigneeID = getUser("assignee")
		data.Engineer, data.EngineerEmail, data.EngineerID = getUser("customfield_10943")
		data.QA, data.QAEmail, data.QAID = getUser("customfield_10641")

		// Helper to extract float fields
		getFloat := func(field string) *float64 {
			if val, ok := issue.Fields[field].(float64); ok {
				return &val
			}
			return nil
		}

		data.StoryPoints = getFloat("customfield_10028")
		data.QAStoryPoints = getFloat("customfield_10603")

		// Extract Status and Resolution
		if st, ok := issue.Fields["status"].(map[string]interface{}); ok {
			data.Status, _ = st["name"].(string)
			if cat, ok := st["statusCategory"].(map[string]interface{}); ok {
				data.StatusCategory, _ = cat["name"].(string)
			}
		}
		if res, ok := issue.Fields["resolution"].(map[string]interface{}); ok {
			data.Resolution, _ = res["name"].(string)
		}

		// Analyze Changelog
		type Transition struct {
			Time   time.Time
			Status string
		}
		transitions := []Transition{}

	if issue.Changelog != nil {
		for _, history := range issue.Changelog.Histories {
			t, err := time.Parse("2006-01-02T15:04:05.000-0700", history.Created)
			if err != nil {
				continue
			}
			for _, item := range history.Items {
				if item.Field == "status" {
					transitions = append(transitions, Transition{Time: t, Status: item.ToString})
					
					// Start Logic: Earliest transition to Start Status
					if startStatuses[item.ToString] {
						if data.Started == nil || t.Before(*data.Started) {
							ts := t
							data.Started = &ts
						}
					}
					// End Logic: Latest transition to End Status
					if endStatuses[item.ToString] {
						if data.Completed == nil || t.After(*data.Completed) {
							ts := t
							data.Completed = &ts
						}
					}
				}
			}
		}
	}

	// Status Category Changed Date Logic to fix "Bulk Move Noise"
	// If current status is Done/Resolved (statusCategory = Done/3), check statuscategorychangedate.
	// If we calculated a Completed time that is much LATER than statuscategorychangedate,
	// it implies the recent transition was a lateral move (Done -> Done), e.g. Done -> Resolved.
	// In that case, the TRUE completion time was the statuscategorychangedate.
	
	// 1. Get statuscategorychangedate
	statusCatDateStr, _ := issue.Fields["statuscategorychangedate"].(string)
	if statusCatDateStr != "" {
		if scTime, err := time.Parse("2006-01-02T15:04:05.000-0700", statusCatDateStr); err == nil {
			// 2. Check if current status category is Done (id 3 or key "done")
			// We already extracted data.StatusCategory name (e.g. "Done").
			// But let's check the ID or key if possible for robustness, or rely on the Name "Done".
			// Standard Jira: "Done" (Green).
			// We can also assume that if completedTime is set, the user considers it Done.
			
			if data.Completed != nil {
				// Check if statusCategory is Done-like.
				isDoneCategory := false
				if st, ok := issue.Fields["status"].(map[string]interface{}); ok {
					if cat, ok := st["statusCategory"].(map[string]interface{}); ok {
						if key, ok := cat["key"].(string); ok && key == "done" {
							isDoneCategory = true
						}
					}
				}

				if isDoneCategory {
					// Compare times. If computed Completed is > scTime by a margin (e.g. 24h), override.
					// Using 1 hour to be safe against minor drifts/migrations.
					if data.Completed.After(scTime.Add(1 * time.Hour)) {
						// The transition we found in changelog (e.g. yesterday) is LATER than when the category changed (years ago).
						// This means the ticket has been "Done" since years ago.
						data.Completed = &scTime
					}
				}
			}
		}
	}
	
	// Determine Initial Status
	currentStatus := "Unknown"
	// Check changelog for first transition's FromString
	if issue.Changelog != nil {
		var earliestTime time.Time
		found := false
		for _, history := range issue.Changelog.Histories {
			t, err := time.Parse("2006-01-02T15:04:05.000-0700", history.Created)
			if err != nil { continue }
			for _, item := range history.Items {
				if item.Field == "status" {
					if !found || t.Before(earliestTime) {
						earliestTime = t
						currentStatus = item.FromString
						found = true
					}
				}
			}
		}
	}
	// Fallback to current status if no history
	if currentStatus == "Unknown" {
		if st, ok := issue.Fields["status"].(map[string]interface{}); ok {
			if name, ok := st["name"].(string); ok {
				currentStatus = name
			}
		}
	}

	// Check if initial status is Start/End
	if startStatuses[currentStatus] {
		if data.Started == nil || data.Created.Before(*data.Started) {
			t := data.Created
			data.Started = &t
		}
	}
	if endStatuses[currentStatus] {
		if data.Completed == nil || data.Created.After(*data.Completed) {
			t := data.Created
			data.Completed = &t
		}
	}

	// Calculate Lead Time & Cycle Time
	if data.Completed != nil && !data.Created.IsZero() {
		diff := data.Completed.Sub(data.Created).Hours() / 24.0
		data.LeadTimeDays = &diff
	}
	if data.Started != nil && data.Completed != nil {
		diff := data.Completed.Sub(*data.Started).Hours() / 24.0
		if diff < 0 { diff = 0 }
		data.CycleTimeDays = &diff
	}

	// Calculate Active Time
	// Sort transitions
	sort.Slice(transitions, func(i, j int) bool {
		return transitions[i].Time.Before(transitions[j].Time)
	})

	activeDuration := 0.0
	lastTime := data.Created
	curr := currentStatus

	for _, tr := range transitions {
		if activeStatuses[curr] {
			activeDuration += tr.Time.Sub(lastTime).Hours()
		}
		curr = tr.Status
		lastTime = tr.Time
	}
	
	endTime := time.Now()
	if data.Completed != nil {
		endTime = *data.Completed
	}
	if lastTime.Before(endTime) {
		if activeStatuses[curr] {
			activeDuration += endTime.Sub(lastTime).Hours()
		}
	}

	val := activeDuration / 24.0
	data.ActiveTimeDays = &val

	// Calculate Age in Current Status
	// lastTime is the start of the current segment (either Created or Last Transition)
	// We calculate age only if the issue is NOT completed (WIP).
	if data.Completed == nil {
		ageDuration := endTime.Sub(lastTime).Hours() / 24.0
		data.AgeInCurrentStatus = &ageDuration
	}

	// Calculate Efficiency
	if data.CycleTimeDays != nil && *data.CycleTimeDays > 0 {
		eff := (*data.ActiveTimeDays / *data.CycleTimeDays) * 100.0
		data.FlowEfficiency = &eff
	} else if data.CycleTimeDays != nil && *data.CycleTimeDays == 0 {
		if *data.ActiveTimeDays > 0 {
			v := 100.0
			data.FlowEfficiency = &v
		} else {
			v := 0.0
			data.FlowEfficiency = &v
		}
	}

	return data
}

func (d *Datasource) getJQLData(issues []jira.Issue) backend.DataResponse {
	var response backend.DataResponse

	frame := data.NewFrame("response",
		data.NewField("Key", nil, []string{}),
		data.NewField("Summary", nil, []string{}),
		data.NewField("Status", nil, []string{}),
		data.NewField("IssueType", nil, []string{}),
		data.NewField("Project", nil, []string{}),
	)

	for _, issue := range issues {
		summary := ""
		if s, ok := issue.Fields["summary"].(string); ok {
			summary = s
		}

		status := ""
		if st, ok := issue.Fields["status"].(map[string]interface{}); ok {
			status, _ = st["name"].(string)
		}

		issueType := ""
		if it, ok := issue.Fields["issuetype"].(map[string]interface{}); ok {
			if name, ok := it["name"].(string); ok {
				issueType = name
			}
		}

		project := ""
		if p, ok := issue.Fields["project"].(map[string]interface{}); ok {
			// Try key, then name
			if key, ok := p["key"].(string); ok {
				project = key
			} else if name, ok := p["name"].(string); ok {
				project = name
			}
		}

		frame.AppendRow(issue.Key, summary, status, issueType, project)
	}

	response.Frames = append(response.Frames, frame)
	return response
}

func (d *Datasource) getChangelogRawData(issues []jira.Issue) backend.DataResponse {
	var response backend.DataResponse
	
	frame := data.NewFrame("response",
		data.NewField("IssueKey", nil, []string{}),
		data.NewField("IssueType", nil, []string{}),
		data.NewField("Created", nil, []time.Time{}),
		data.NewField("field", nil, []string{}),
		data.NewField("fromValue", nil, []string{}),
		data.NewField("toValue", nil, []string{}),
	)

	for _, issue := range issues {
		if issue.Changelog == nil {
			continue
		}
		
		issueType := "Unknown"
		if it, ok := issue.Fields["issuetype"].(map[string]interface{}); ok {
			if name, ok := it["name"].(string); ok {
				issueType = name
			}
		}

		for _, history := range issue.Changelog.Histories {
			createdTime, err := time.Parse("2006-01-02T15:04:05.000-0700", history.Created)
			if err != nil {
				continue
			}

			for _, item := range history.Items {
				frame.AppendRow(
					issue.Key,
					issueType,
					createdTime,
					item.Field,
					item.FromString,
					item.ToString,
				)
			}
		}
	}

	response.Frames = append(response.Frames, frame)
	return response
}

func (d *Datasource) getFlowData(issues []jira.Issue, qm queryModel, timeRange backend.TimeRange, baseURL string) backend.DataResponse {
	var response backend.DataResponse

	// Parse Statuses
	parseStatuses := func(s string) map[string]bool {
		m := make(map[string]bool)
		raw := strings.Trim(s, "{}")
		for _, p := range strings.Split(raw, ",") {
			t := strings.TrimSpace(p)
			if t != "" {
				m[t] = true
			}
		}
		return m
	}
	startS := parseStatuses(qm.StartStatus)
	endS := parseStatuses(qm.EndStatus)
	activeS := parseStatuses(qm.ActiveStatuses)

	// Analyze all issues
	var analyzed []issueData
	var cycleTimes []float64

	for _, issue := range issues {
		a := d.analyzeIssue(issue, startS, endS, activeS)
		
		// Filter out issues that were completed BEFORE the requested time window.
		// This removes "noise" from old tickets that were recently updated (commented, labeled) but finished long ago.
		// We keep issues that are:
		// 1. WIP (Completed is nil)
		// 2. Completed WITHIN the window (Completed >= From)
		// 3. Completed AFTER the window? (Unlikely given updated filter, but valid to keep)
		if a.Completed != nil && a.Completed.Before(timeRange.From) {
			continue
		}

		analyzed = append(analyzed, a)
		if a.CycleTimeDays != nil {
			cycleTimes = append(cycleTimes, *a.CycleTimeDays)
		}
	}

	// Calculate Quantile
	quantileValue := 0.0
	if len(cycleTimes) > 0 {
		sort.Float64s(cycleTimes)
		pos := (qm.Quantile / 100.0) * float64(len(cycleTimes)-1)
		base := int(pos)
		rest := pos - float64(base)
		if base+1 < len(cycleTimes) {
			quantileValue = cycleTimes[base] + rest*(cycleTimes[base+1]-cycleTimes[base])
		} else {
			quantileValue = cycleTimes[base]
		}
	}

	// Construct Frame based on Metric type
	// New Issue Flow Format
	frame := data.NewFrame("response",
		data.NewField("IssueKey", nil, []string{}),
		data.NewField("Summary", nil, []string{}),
		data.NewField("IssueType", nil, []string{}),
		data.NewField("Project", nil, []string{}),
		data.NewField("Created", nil, []time.Time{}),
		data.NewField("Started", nil, []*time.Time{}),
		data.NewField("Completed", nil, []*time.Time{}),
		data.NewField("LeadTimeDays", nil, []*float64{}),
		data.NewField("CycleTimeDays", nil, []*float64{}),
		data.NewField("ActiveTimeDays", nil, []*float64{}),
		data.NewField("FlowEfficiency", nil, []*float64{}),
		data.NewField("Quantile", nil, []*float64{}),
		data.NewField("Assignee", nil, []string{}),
		data.NewField("AssigneeEmail", nil, []string{}),
		data.NewField("AssigneeID", nil, []string{}),
		data.NewField("Engineer", nil, []string{}),
		data.NewField("EngineerEmail", nil, []string{}),
		data.NewField("EngineerID", nil, []string{}),
		data.NewField("QA", nil, []string{}),
		data.NewField("QAEmail", nil, []string{}),
		data.NewField("QAID", nil, []string{}),
		data.NewField("StoryPoints", nil, []*float64{}),
		data.NewField("QAStoryPoints", nil, []*float64{}),
		data.NewField("Status", nil, []string{}),
		data.NewField("StatusCategory", nil, []string{}),
		data.NewField("Resolution", nil, []string{}),
		data.NewField("AgeInCurrentStatus", nil, []*float64{}),
	)

	for _, a := range analyzed {
		// Extract fields
		summary := ""
		if s, ok := a.Issue.Fields["summary"].(string); ok {
			summary = s
		}
		issueType := "Unknown"
		if it, ok := a.Issue.Fields["issuetype"].(map[string]interface{}); ok {
			if name, ok := it["name"].(string); ok {
				issueType = name
			}
		}
		project := ""
		if p, ok := a.Issue.Fields["project"].(map[string]interface{}); ok {
			if key, ok := p["key"].(string); ok {
				project = key
			} else if name, ok := p["name"].(string); ok {
			project = name
			}
		}

		frame.AppendRow(
			a.Issue.Key,
			summary,
			issueType,
			project,
			a.Created,
			a.Started,
			a.Completed,
			a.LeadTimeDays,
			a.CycleTimeDays,
			a.ActiveTimeDays,
			a.FlowEfficiency,
			&quantileValue,
			a.Assignee,
			a.AssigneeEmail,
			a.AssigneeID,
			a.Engineer,
			a.EngineerEmail,
			a.EngineerID,
			a.QA,
			a.QAEmail,
			a.QAID,
			a.StoryPoints,
			a.QAStoryPoints,
			a.Status,
			a.StatusCategory,
			a.Resolution,
			a.AgeInCurrentStatus,
		)
	}
	response.Frames = append(response.Frames, frame)

	return response
}

func (d *Datasource) CheckHealth(_ context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	res := &backend.CheckHealthResult{}
	config, err := models.LoadPluginSettings(*req.PluginContext.DataSourceInstanceSettings)

	if err != nil {
		res.Status = backend.HealthStatusError
		res.Message = "Unable to load settings"
		return res, nil
	}

	if config.Secrets.Token == "" {
		res.Status = backend.HealthStatusError
		res.Message = "API Token is missing"
		return res, nil
	}

	client := jira.NewClient(config.URL, config.Username, config.Secrets.Token)
	err = client.Myself()
	if err != nil {
		res.Status = backend.HealthStatusError
		res.Message = fmt.Sprintf("Jira connection failed: %s", err.Error())
		return res, nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

