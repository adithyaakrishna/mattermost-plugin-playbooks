package app

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/mattermost/mattermost-server/v5/model"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-plugin-api/cluster"
)

const (
	StatusReported = "Reported"
	StatusActive   = "Active"
	StatusResolved = "Resolved"
	StatusArchived = "Archived"
)

// PlaybookRun holds the detailed information of an incident.
//
// NOTE: when adding a column to the db, search for "When adding an Incident column" to see where
// that column needs to be added in the sqlstore code.
type PlaybookRun struct {
	ID                                   string          `json:"id"`
	Name                                 string          `json:"name"` // Retrieved from incident channel
	Description                          string          `json:"description"`
	OwnerUserID                          string          `json:"owner_user_id"`
	ReporterUserID                       string          `json:"reporter_user_id"`
	TeamID                               string          `json:"team_id"`
	ChannelID                            string          `json:"channel_id"`
	CreateAt                             int64           `json:"create_at"` // Retrieved from incident channel
	EndAt                                int64           `json:"end_at"`
	DeleteAt                             int64           `json:"delete_at"` // Retrieved from incidet channel
	ActiveStage                          int             `json:"active_stage"`
	ActiveStageTitle                     string          `json:"active_stage_title"`
	PostID                               string          `json:"post_id"`
	PlaybookID                           string          `json:"playbook_id"`
	Checklists                           []Checklist     `json:"checklists"`
	StatusPosts                          []StatusPost    `json:"status_posts"`
	CurrentStatus                        string          `json:"current_status"`
	ReminderPostID                       string          `json:"reminder_post_id"`
	PreviousReminder                     time.Duration   `json:"previous_reminder"`
	BroadcastChannelID                   string          `json:"broadcast_channel_id"`
	ReminderMessageTemplate              string          `json:"reminder_message_template"`
	InvitedUserIDs                       []string        `json:"invited_user_ids"`
	InvitedGroupIDs                      []string        `json:"invited_group_ids"`
	TimelineEvents                       []TimelineEvent `json:"timeline_events"`
	DefaultOwnerID                       string          `json:"default_owner_id"`
	AnnouncementChannelID                string          `json:"announcement_channel_id"`
	WebhookOnCreationURL                 string          `json:"webhook_on_creation_url"`
	WebhookOnStatusUpdateURL             string          `json:"webhook_on_status_update_url"`
	Retrospective                        string          `json:"retrospective"`
	RetrospectivePublishedAt             int64           `json:"retrospective_published_at"` // The last time a retrospective was published. 0 if never published.
	RetrospectiveWasCanceled             bool            `json:"retrospective_was_canceled"`
	RetrospectiveReminderIntervalSeconds int64           `json:"retrospective_reminder_interval_seconds"`
	MessageOnJoin                        string          `json:"message_on_join"`
}

func (i *PlaybookRun) Clone() *PlaybookRun {
	newPlaybookRun := *i
	var newChecklists []Checklist
	for _, c := range i.Checklists {
		newChecklists = append(newChecklists, c.Clone())
	}
	newPlaybookRun.Checklists = newChecklists

	newPlaybookRun.StatusPosts = append([]StatusPost(nil), i.StatusPosts...)
	newPlaybookRun.TimelineEvents = append([]TimelineEvent(nil), i.TimelineEvents...)
	newPlaybookRun.InvitedUserIDs = append([]string(nil), i.InvitedUserIDs...)
	newPlaybookRun.InvitedGroupIDs = append([]string(nil), i.InvitedGroupIDs...)

	return &newPlaybookRun
}

func (i *PlaybookRun) MarshalJSON() ([]byte, error) {
	type Alias PlaybookRun

	old := (*Alias)(i.Clone())
	// replace nils with empty slices for the frontend
	if old.Checklists == nil {
		old.Checklists = []Checklist{}
	}
	for j, cl := range old.Checklists {
		if cl.Items == nil {
			old.Checklists[j].Items = []ChecklistItem{}
		}
	}
	if old.StatusPosts == nil {
		old.StatusPosts = []StatusPost{}
	}
	if old.InvitedUserIDs == nil {
		old.InvitedUserIDs = []string{}
	}
	if old.InvitedGroupIDs == nil {
		old.InvitedGroupIDs = []string{}
	}
	if old.TimelineEvents == nil {
		old.TimelineEvents = []TimelineEvent{}
	}

	return json.Marshal(old)
}

func (i *PlaybookRun) IsActive() bool {
	currentStatus := i.CurrentStatus
	return currentStatus != StatusResolved && currentStatus != StatusArchived
}

func (i *PlaybookRun) ResolvedAt() int64 {
	// Backwards compatibility for incidents with old status updates
	if len(i.StatusPosts) > 0 && i.StatusPosts[len(i.StatusPosts)-1].Status == "" {
		return i.EndAt
	}

	var resolvedPost *StatusPost
	for j := len(i.StatusPosts) - 1; j >= 0; j-- {
		if i.StatusPosts[j].DeleteAt != 0 {
			continue
		}
		if i.StatusPosts[j].Status != StatusResolved && i.StatusPosts[j].Status != StatusArchived {
			break
		}

		resolvedPost = &i.StatusPosts[j]
	}

	if resolvedPost == nil {
		return 0
	}

	return resolvedPost.CreateAt
}

type StatusPost struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	CreateAt int64  `json:"create_at"`
	DeleteAt int64  `json:"delete_at"`
}

type UpdateOptions struct {
}

// StatusUpdateOptions encapsulates the fields that can be set when updating an incident's status
// NOTE: changes made to this should be reflected in the client/incident StatusUpdateOptions struct
type StatusUpdateOptions struct {
	Status      string        `json:"status"`
	Description string        `json:"description"`
	Message     string        `json:"message"`
	Reminder    time.Duration `json:"reminder"`
}

// Metadata tracks ancillary metadata about an incident.
type Metadata struct {
	ChannelName        string `json:"channel_name"`
	ChannelDisplayName string `json:"channel_display_name"`
	TeamName           string `json:"team_name"`
	NumMembers         int64  `json:"num_members"`
	TotalPosts         int64  `json:"total_posts"`
}

type timelineEventType string

const (
	PlaybookRunCreated     timelineEventType = "incident_created"
	TaskStateModified      timelineEventType = "task_state_modified"
	StatusUpdated          timelineEventType = "status_updated"
	OwnerChanged           timelineEventType = "owner_changed"
	AssigneeChanged        timelineEventType = "assignee_changed"
	RanSlashCommand        timelineEventType = "ran_slash_command"
	EventFromPost          timelineEventType = "event_from_post"
	UserJoinedLeft         timelineEventType = "user_joined_left"
	PublishedRetrospective timelineEventType = "published_retrospective"
	CanceledRetrospective  timelineEventType = "canceled_retrospective"
)

type TimelineEvent struct {
	ID            string            `json:"id"`
	PlaybookRunID string            `json:"incident_id"`
	CreateAt      int64             `json:"create_at"`
	DeleteAt      int64             `json:"delete_at"`
	EventAt       int64             `json:"event_at"`
	EventType     timelineEventType `json:"event_type"`
	Summary       string            `json:"summary"`
	Details       string            `json:"details"`
	PostID        string            `json:"post_id"`
	SubjectUserID string            `json:"subject_user_id"`
	CreatorUserID string            `json:"creator_user_id"`
}

// GetPlaybookRunsResults collects the results of the GetPlaybookRuns call: the list of PlaybookRuns matching
// the HeaderFilterOptions, and the TotalCount of the matching incidents before paging was applied.
type GetPlaybookRunsResults struct {
	TotalCount int           `json:"total_count"`
	PageCount  int           `json:"page_count"`
	HasMore    bool          `json:"has_more"`
	Items      []PlaybookRun `json:"items"`
}

type SQLStatusPost struct {
	PlaybookRunID string
	PostID        string
	Status        string
	EndAt         int64
}

func (r GetPlaybookRunsResults) Clone() GetPlaybookRunsResults {
	newGetPlaybookRunsResults := r

	newGetPlaybookRunsResults.Items = make([]PlaybookRun, 0, len(r.Items))
	for _, i := range r.Items {
		newGetPlaybookRunsResults.Items = append(newGetPlaybookRunsResults.Items, *i.Clone())
	}

	return newGetPlaybookRunsResults
}

func (r GetPlaybookRunsResults) MarshalJSON() ([]byte, error) {
	type Alias GetPlaybookRunsResults

	old := Alias(r.Clone())

	// replace nils with empty slices for the frontend
	if old.Items == nil {
		old.Items = []PlaybookRun{}
	}

	return json.Marshal(old)
}

// OwnerInfo holds the summary information of a owner.
type OwnerInfo struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// DialogState holds the start incident interactive dialog's state as it appears in the client
// and is submitted back to the server.
type DialogState struct {
	PostID   string `json:"post_id"`
	ClientID string `json:"client_id"`
}

type DialogStateAddToTimeline struct {
	PostID string `json:"post_id"`
}

// PlaybookRunService is the incident/service interface.
type PlaybookRunService interface {
	// GetPlaybookRuns returns filtered incidents and the total count before paging.
	GetPlaybookRuns(requesterInfo RequesterInfo, options PlaybookRunFilterOptions) (*GetPlaybookRunsResults, error)

	// CreatePlaybookRun creates a new incident. userID is the user who initiated the CreatePlaybookRun.
	CreatePlaybookRun(incident *PlaybookRun, playbook *Playbook, userID string, public bool) (*PlaybookRun, error)

	// OpenCreatePlaybookRunDialog opens an interactive dialog to start a new incident.
	OpenCreatePlaybookRunDialog(teamID, ownerID, triggerID, postID, clientID string, playbooks []Playbook, isMobileApp bool) error

	// OpenUpdateStatusDialog opens an interactive dialog so the user can update the incident's status.
	OpenUpdateStatusDialog(incidentID, triggerID string) error

	// OpenAddToTimelineDialog opens an interactive dialog so the user can add a post to the incident timeline.
	OpenAddToTimelineDialog(requesterInfo RequesterInfo, postID, teamID, triggerID string) error

	// OpenAddChecklistItemDialog opens an interactive dialog so the user can add a post to the incident timeline.
	OpenAddChecklistItemDialog(triggerID, incidentID string, checklist int) error

	// AddPostToTimeline adds an event based on a post to an incident's timeline.
	AddPostToTimeline(incidentID, userID, postID, summary string) error

	// RemoveTimelineEvent removes the timeline event (sets the DeleteAt to the current time).
	RemoveTimelineEvent(incidentID, userID, eventID string) error

	// UpdateStatus updates an incident's status.
	UpdateStatus(incidentID, userID string, options StatusUpdateOptions) error

	// GetPlaybookRun gets an incident by ID. Returns error if it could not be found.
	GetPlaybookRun(incidentID string) (*PlaybookRun, error)

	// GetPlaybookRunMetadata gets ancillary metadata about an incident.
	GetPlaybookRunMetadata(incidentID string) (*Metadata, error)

	// GetPlaybookRunIDForChannel get the incidentID associated with this channel. Returns ErrNotFound
	// if there is no incident associated with this channel.
	GetPlaybookRunIDForChannel(channelID string) (string, error)

	// GetOwners returns all the owners of incidents selected
	GetOwners(requesterInfo RequesterInfo, options PlaybookRunFilterOptions) ([]OwnerInfo, error)

	// IsOwner returns true if the userID is the owner for incidentID.
	IsOwner(incidentID string, userID string) bool

	// ChangeOwner processes a request from userID to change the owner for incidentID
	// to ownerID. Changing to the same ownerID is a no-op.
	ChangeOwner(incidentID string, userID string, ownerID string) error

	// ModifyCheckedState modifies the state of the specified checklist item
	// Idempotent, will not perform any actions if the checklist item is already in the specified state
	ModifyCheckedState(incidentID, userID, newState string, checklistNumber int, itemNumber int) error

	// ToggleCheckedState checks or unchecks the specified checklist item
	ToggleCheckedState(incidentID, userID string, checklistNumber, itemNumber int) error

	// SetAssignee sets the assignee for the specified checklist item
	// Idempotent, will not perform any actions if the checklist item is already assigned to assigneeID
	SetAssignee(incidentID, userID, assigneeID string, checklistNumber, itemNumber int) error

	// RunChecklistItemSlashCommand executes the slash command associated with the specified checklist item.
	RunChecklistItemSlashCommand(incidentID, userID string, checklistNumber, itemNumber int) (string, error)

	// AddChecklistItem adds an item to the specified checklist
	AddChecklistItem(incidentID, userID string, checklistNumber int, checklistItem ChecklistItem) error

	// RemoveChecklistItem removes an item from the specified checklist
	RemoveChecklistItem(incidentID, userID string, checklistNumber int, itemNumber int) error

	// EditChecklistItem changes the title, command and description of a specified checklist item.
	EditChecklistItem(incidentID, userID string, checklistNumber int, itemNumber int, newTitle, newCommand, newDescription string) error

	// MoveChecklistItem moves a checklist item from one position to anouther
	MoveChecklistItem(incidentID, userID string, checklistNumber int, itemNumber int, newLocation int) error

	// GetChecklistItemAutocomplete returns the list of checklist items for incidentID to be used in autocomplete
	GetChecklistItemAutocomplete(incidentID string) ([]model.AutocompleteListItem, error)

	// GetChecklistAutocomplete returns the list of checklists for incidentID to be used in autocomplete
	GetChecklistAutocomplete(incidentID string) ([]model.AutocompleteListItem, error)

	// NukeDB removes all incident related data.
	NukeDB() error

	// SetReminder sets a reminder. After timeInMinutes in the future, the owner will be
	// reminded to update the incident's status.
	SetReminder(incidentID string, timeInMinutes time.Duration) error

	// RemoveReminder removes the pending reminder for incidentID (if any).
	RemoveReminder(incidentID string)

	// HandleReminder is the handler for all reminder events.
	HandleReminder(key string)

	// RemoveReminderPost will remove the reminder in the incident channel (if any).
	RemoveReminderPost(incidentID string) error

	// ChangeCreationDate changes the creation date of the specified incident.
	ChangeCreationDate(incidentID string, creationTimestamp time.Time) error

	// UserHasJoinedChannel is called when userID has joined channelID. If actorID is not blank, userID
	// was invited by actorID.
	UserHasJoinedChannel(userID, channelID, actorID string)

	// UserHasLeftChannel is called when userID has left channelID. If actorID is not blank, userID
	// was removed from the channel by actorID.
	UserHasLeftChannel(userID, channelID, actorID string)

	// UpdateRetrospective updates the retrospective for the given incident.
	UpdateRetrospective(incidentID, userID, newRetrospective string) error

	// PublishRetrospective publishes the retrospective.
	PublishRetrospective(incidentID, text, userID string) error

	// CancelRetrospective cancels the retrospective.
	CancelRetrospective(incidentID, userID string) error

	// CheckAndSendMessageOnJoin checks if userID has viewed channelID and sends
	// incident.MessageOnJoin if it exists. Returns true if the message was sent.
	CheckAndSendMessageOnJoin(userID, incidentID, channelID string) bool
}

// PlaybookRunStore defines the methods the PlaybookRunServiceImpl needs from the interfaceStore.
type PlaybookRunStore interface {
	// GetPlaybookRuns returns filtered incidents and the total count before paging.
	GetPlaybookRuns(requesterInfo RequesterInfo, options PlaybookRunFilterOptions) (*GetPlaybookRunsResults, error)

	// CreatePlaybookRun creates a new incident. If incident has an ID, that ID will be used.
	CreatePlaybookRun(incident *PlaybookRun) (*PlaybookRun, error)

	// UpdatePlaybookRun updates an incident.
	UpdatePlaybookRun(incident *PlaybookRun) error

	// UpdateStatus updates the status of an incident.
	UpdateStatus(statusPost *SQLStatusPost) error

	// GetTimelineEvent returns the timeline event for incidentID by the timeline event ID.
	GetTimelineEvent(incidentID, eventID string) (*TimelineEvent, error)

	// CreateTimelineEvent inserts the timeline event into the DB and returns the new event ID
	CreateTimelineEvent(event *TimelineEvent) (*TimelineEvent, error)

	// UpdateTimelineEvent updates an existing timeline event
	UpdateTimelineEvent(event *TimelineEvent) error

	// GetPlaybookRun gets an incident by ID.
	GetPlaybookRun(incidentID string) (*PlaybookRun, error)

	// GetPlaybookRunByChannel gets an incident associated with the given channel id.
	GetPlaybookRunIDForChannel(channelID string) (string, error)

	// GetAllPlaybookRunMembersCount returns the count of all members of the
	// incident associated with the given channel id since the beginning of the
	// incident, excluding bots.
	GetAllPlaybookRunMembersCount(channelID string) (int64, error)

	// GetOwners returns the owners of the incidents selected by options
	GetOwners(requesterInfo RequesterInfo, options PlaybookRunFilterOptions) ([]OwnerInfo, error)

	// NukeDB removes all incident related data.
	NukeDB() error

	// ChangeCreationDate changes the creation date of the specified incident.
	ChangeCreationDate(incidentID string, creationTimestamp time.Time) error

	// HasViewedChannel returns true if userID has viewed channelID
	HasViewedChannel(userID, channelID string) bool

	// SetViewedChannel records that userID has viewed channelID. NOTE: does not check if there is already a
	// record of that userID/channelID (i.e., will create duplicate rows)
	SetViewedChannel(userID, channelID string) error
}

// PlaybookRunTelemetry defines the methods that the PlaybookRunServiceImpl needs from the RudderTelemetry.
// Unless otherwise noted, userID is the user initiating the event.
type PlaybookRunTelemetry interface {
	// CreatePlaybookRun tracks the creation of a new incident.
	CreatePlaybookRun(incident *PlaybookRun, userID string, public bool)

	// EndPlaybookRun tracks the end of an incident.
	EndPlaybookRun(incident *PlaybookRun, userID string)

	// RestartPlaybookRun tracks the restart of an incident.
	RestartPlaybookRun(incident *PlaybookRun, userID string)

	// ChangeOwner tracks changes in owner.
	ChangeOwner(incident *PlaybookRun, userID string)

	// UpdateStatus tracks when an incident's status has been updated
	UpdateStatus(incident *PlaybookRun, userID string)

	// FrontendTelemetryForPlaybookRun tracks an event originating from the frontend
	FrontendTelemetryForPlaybookRun(incident *PlaybookRun, userID, action string)

	// AddPostToTimeline tracks userID creating a timeline event from a post.
	AddPostToTimeline(incident *PlaybookRun, userID string)

	// RemoveTimelineEvent tracks userID removing a timeline event.
	RemoveTimelineEvent(incident *PlaybookRun, userID string)

	// ModifyCheckedState tracks the checking and unchecking of items.
	ModifyCheckedState(incidentID, userID string, task ChecklistItem, wasOwner bool)

	// SetAssignee tracks the changing of an assignee on an item.
	SetAssignee(incidentID, userID string, task ChecklistItem)

	// AddTask tracks the creation of a new checklist item.
	AddTask(incidentID, userID string, task ChecklistItem)

	// RemoveTask tracks the removal of a checklist item.
	RemoveTask(incidentID, userID string, task ChecklistItem)

	// RenameTask tracks the update of a checklist item.
	RenameTask(incidentID, userID string, task ChecklistItem)

	// MoveTask tracks the unchecking of checked item.
	MoveTask(incidentID, userID string, task ChecklistItem)

	// RunTaskSlashCommand tracks the execution of a slash command attached to
	// a checklist item.
	RunTaskSlashCommand(incidentID, userID string, task ChecklistItem)

	// UpdateRetrospective event
	UpdateRetrospective(incident *PlaybookRun, userID string)

	// PublishRetrospective event
	PublishRetrospective(incident *PlaybookRun, userID string)
}

type JobOnceScheduler interface {
	Start() error
	SetCallback(callback func(string)) error
	ListScheduledJobs() ([]cluster.JobOnceMetadata, error)
	ScheduleOnce(key string, runAt time.Time) (*cluster.JobOnce, error)
	Cancel(key string)
}

const PerPageDefault = 1000

// PlaybookRunFilterOptions specifies the optional parameters when getting headers.
type PlaybookRunFilterOptions struct {
	// Gets all the headers with this TeamID.
	TeamID string `url:"team_id,omitempty"`

	// Pagination options.
	Page    int `url:"page,omitempty"`
	PerPage int `url:"per_page,omitempty"`

	// Sort sorts by this header field in json format (eg, "create_at", "end_at", "name", etc.);
	// defaults to "create_at".
	Sort SortField `url:"sort,omitempty"`

	// Direction orders by ascending or descending, defaulting to ascending.
	Direction SortDirection `url:"direction,omitempty"`

	// Status filters by current status
	Status string

	// Statuses filters by all statuses in the list (inclusive)
	Statuses []string

	// OwnerID filters by owner's Mattermost user ID. Defaults to blank (no filter).
	OwnerID string `url:"owner_user_id,omitempty"`

	// MemberID filters incidents that have this member. Defaults to blank (no filter).
	MemberID string `url:"member_id,omitempty"`

	// SearchTerm returns results of the search term and respecting the other header filter options.
	// The search term acts as a filter and respects the Sort and Direction fields (i.e., results are
	// not returned in relevance order).
	SearchTerm string `url:"search_term,omitempty"`

	// PlaybookID filters incidents that are derived from this playbook id.
	// Defaults to blank (no filter).
	PlaybookID string `url:"playbook_id,omitempty"`
}

// Clone duplicates the given options.
func (o *PlaybookRunFilterOptions) Clone() PlaybookRunFilterOptions {
	newPlaybookRunFilterOptions := *o
	newPlaybookRunFilterOptions.Statuses = append([]string{}, o.Statuses...)

	return newPlaybookRunFilterOptions
}

// Validate returns a new, validated filter options or returns an error if invalid.
func (o PlaybookRunFilterOptions) Validate() (PlaybookRunFilterOptions, error) {
	options := o.Clone()

	if options.PerPage <= 0 {
		options.PerPage = PerPageDefault
	}

	options.Sort = SortField(strings.ToLower(string(options.Sort)))
	switch options.Sort {
	case SortByCreateAt:
	case SortByID:
	case SortByName:
	case SortByOwnerUserID:
	case SortByTeamID:
	case SortByEndAt:
	case SortByStatus:
	case "": // default
		options.Sort = SortByCreateAt
	default:
		return PlaybookRunFilterOptions{}, errors.Errorf("unsupported sort '%s'", options.Sort)
	}

	options.Direction = SortDirection(strings.ToUpper(string(options.Direction)))
	switch options.Direction {
	case DirectionAsc:
	case DirectionDesc:
	case "": //default
		options.Direction = DirectionAsc
	default:
		return PlaybookRunFilterOptions{}, errors.Errorf("unsupported direction '%s'", options.Direction)
	}

	if options.TeamID != "" && !model.IsValidId(options.TeamID) {
		return PlaybookRunFilterOptions{}, errors.New("bad parameter 'team_id': must be 26 characters or blank")
	}

	if options.OwnerID != "" && !model.IsValidId(options.OwnerID) {
		return PlaybookRunFilterOptions{}, errors.New("bad parameter 'owner_id': must be 26 characters or blank")
	}

	if options.MemberID != "" && !model.IsValidId(options.MemberID) {
		return PlaybookRunFilterOptions{}, errors.New("bad parameter 'member_id': must be 26 characters or blank")
	}

	if options.PlaybookID != "" && !model.IsValidId(options.PlaybookID) {
		return PlaybookRunFilterOptions{}, errors.New("bad parameter 'playbook_id': must be 26 characters or blank")
	}

	return options, nil
}