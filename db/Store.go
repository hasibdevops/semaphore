package db

import (
	"errors"
	log "github.com/Sirupsen/logrus"
	"reflect"
	"time"
)

const databaseTimeFormat = "2006-01-02T15:04:05:99Z"

// GetParsedTime returns the timestamp as it will retrieved from the database
// This allows us to create timestamp consistency on return values from create requests
func GetParsedTime(t time.Time) time.Time {
	parsedTime, err := time.Parse(databaseTimeFormat, t.Format(databaseTimeFormat))
	if err != nil {
		log.Error(err)
	}
	return parsedTime
}

type RetrieveQueryParams struct {
	Offset       int
	Count        int
	SortBy       string
	SortInverted bool
}

type ObjectScope int

type ObjectProperties struct {
	TableName           string
	IsGlobal            bool // doesn't belong to other table, for example to project or user.
	ForeignColumnSuffix string
	PrimaryColumnName   string
	SortableColumns     []string
	SortInverted        bool
	Type                reflect.Type
}

var ErrNotFound = errors.New("no rows in result set")
var ErrInvalidOperation = errors.New("invalid operation")

func ValidateUsername(login string) error {
	return nil
}

type Transaction interface{}

type Store interface {
	Connect() error
	Close() error
	Migrate() error

	GetEnvironment(projectID int, environmentID int) (Environment, error)
	GetEnvironments(projectID int, params RetrieveQueryParams) ([]Environment, error)
	UpdateEnvironment(env Environment) error
	CreateEnvironment(env Environment) (Environment, error)
	DeleteEnvironment(projectID int, templateID int) error
	DeleteEnvironmentSoft(projectID int, templateID int) error

	GetInventory(projectID int, inventoryID int) (Inventory, error)
	GetInventories(projectID int, params RetrieveQueryParams) ([]Inventory, error)
	UpdateInventory(inventory Inventory) error
	CreateInventory(inventory Inventory) (Inventory, error)
	DeleteInventory(projectID int, inventoryID int) error
	DeleteInventorySoft(projectID int, inventoryID int) error

	GetRepository(projectID int, repositoryID int) (Repository, error)
	GetRepositories(projectID int, params RetrieveQueryParams) ([]Repository, error)
	UpdateRepository(repository Repository) error
	CreateRepository(repository Repository) (Repository, error)
	DeleteRepository(projectID int, repositoryID int) error
	DeleteRepositorySoft(projectID int, repositoryID int) error

	GetAccessKey(projectID int, accessKeyID int) (AccessKey, error)
	GetAccessKeys(projectID int, params RetrieveQueryParams) ([]AccessKey, error)

	UpdateAccessKey(accessKey AccessKey) error
	CreateAccessKey(accessKey AccessKey) (AccessKey, error)
	DeleteAccessKey(projectID int, accessKeyID int) error
	DeleteAccessKeySoft(projectID int, accessKeyID int) error

	GetUsers(params RetrieveQueryParams) ([]User, error)
	CreateUserWithoutPassword(user User) (User, error)
	CreateUser(user UserWithPwd) (User, error)
	DeleteUser(userID int) error

	// UpdateUser updates all fields of the entity except Pwd.
	// Pwd should be present of you want update user password. Empty Pwd ignored.
	UpdateUser(user UserWithPwd) error
	SetUserPassword(userID int, password string) error
	GetUser(userID int) (User, error)
	GetUserByLoginOrEmail(login string, email string) (User, error)

	GetProject(projectID int) (Project, error)
	GetProjects(userID int) ([]Project, error)
	CreateProject(project Project) (Project, error)
	DeleteProject(projectID int) error
	UpdateProject(project Project) error

	GetTemplates(projectID int, params RetrieveQueryParams) ([]Template, error)
	CreateTemplate(template Template) (Template, error)
	UpdateTemplate(template Template) error
	GetTemplate(projectID int, templateID int) (Template, error)
	DeleteTemplate(projectID int, templateID int) error

	GetSchedules() ([]Schedule, error)
	GetTemplateSchedules(projectID int, templateID int) ([]Schedule, error)
	CreateSchedule(schedule Schedule) (Schedule, error)
	UpdateSchedule(schedule Schedule) error
	GetSchedule(projectID int, scheduleID int) (Schedule, error)
	DeleteSchedule(projectID int, scheduleID int) error

	GetProjectUsers(projectID int, params RetrieveQueryParams) ([]User, error)
	CreateProjectUser(projectUser ProjectUser) (ProjectUser, error)
	DeleteProjectUser(projectID int, userID int) error
	GetProjectUser(projectID int, userID int) (ProjectUser, error)
	UpdateProjectUser(projectUser ProjectUser) error

	CreateEvent(event Event) (Event, error)
	GetUserEvents(userID int, params RetrieveQueryParams) ([]Event, error)
	GetEvents(projectID int, params RetrieveQueryParams) ([]Event, error)

	GetAPITokens(userID int) ([]APIToken, error)
	CreateAPIToken(token APIToken) (APIToken, error)
	GetAPIToken(tokenID string) (APIToken, error)
	ExpireAPIToken(userID int, tokenID string) error

	GetSession(userID int, sessionID int) (Session, error)
	CreateSession(session Session) (Session, error)
	ExpireSession(userID int, sessionID int) error
	TouchSession(userID int, sessionID int) error

	CreateTask(task Task) (Task, error)
	UpdateTask(task Task) error

	GetTemplateTasks(projectID int, templateID int, params RetrieveQueryParams) ([]TaskWithTpl, error)
	GetProjectTasks(projectID int, params RetrieveQueryParams) ([]TaskWithTpl, error)
	GetTask(projectID int, taskID int) (Task, error)
	DeleteTaskWithOutputs(projectID int, taskID int) error
	GetTaskOutputs(projectID int, taskID int) ([]TaskOutput, error)
	CreateTaskOutput(output TaskOutput) (TaskOutput, error)
}

func FillTemplate(d Store, template *Template) (err error) {
	if template.VaultKeyID != nil {
		template.VaultKey, err = d.GetAccessKey(template.ProjectID, *template.VaultKeyID)
	}
	return
}

func FillInventory(d Store, inventory *Inventory) (err error) {
	if inventory.SSHKeyID != nil {
		inventory.SSHKey, err = d.GetAccessKey(inventory.ProjectID, *inventory.SSHKeyID)
	}

	if err != nil {
		return
	}

	if inventory.BecomeKeyID != nil {
		inventory.BecomeKey, err = d.GetAccessKey(inventory.ProjectID, *inventory.BecomeKeyID)
	}

	return
}

func FillEvents(d Store, events []Event) (err error) {
	usernames := make(map[int]string)

	for i, evt := range events {
		var objName string
		objName, err = getEventObjectName(d, evt)

		if err != nil {
			return
		}

		if objName != "" {
			events[i].ObjectName = objName
		}

		if evt.UserID == nil {
			continue
		}

		var username string

		username, ok := usernames[*evt.UserID]

		if !ok {
			username, err = getEventUsername(d, evt)

			if err != nil {
				return
			}

			if username == "" {
				continue
			}

			usernames[*evt.UserID] = username
		}

		events[i].Username = &username
	}

	return
}

func getEventObjectName(d Store, evt Event) (string, error) {
	if evt.ObjectID == nil || evt.ObjectType == nil {
		return "", nil
	}
	switch *evt.ObjectType {
	case "task":
		task, err := d.GetTask(*evt.ProjectID, *evt.ObjectID)
		if err != nil {
			return "", err
		}
		return task.Playbook, nil
	default:
		return "", nil
	}
}

func getEventUsername(d Store, evt Event) (username string, err error) {
	if evt.UserID == nil {
		return "", nil
	}

	user, err := d.GetUser(*evt.UserID)

	if err != nil {
		return "", err
	}

	return user.Username, nil
}

var AccessKeyProps = ObjectProperties{
	TableName:           "access_key",
	SortableColumns:     []string{"name", "type"},
	ForeignColumnSuffix: "key_id",
	PrimaryColumnName:   "id",
	Type:                reflect.TypeOf(AccessKey{}),
}

var EnvironmentProps = ObjectProperties{
	TableName:           "project__environment",
	SortableColumns:     []string{"name"},
	ForeignColumnSuffix: "environment_id",
	PrimaryColumnName:   "id",
	Type:                reflect.TypeOf(Environment{}),
}

var InventoryProps = ObjectProperties{
	TableName:           "project__inventory",
	SortableColumns:     []string{"name"},
	ForeignColumnSuffix: "inventory_id",
	PrimaryColumnName:   "id",
	Type:                reflect.TypeOf(Inventory{}),
}

var RepositoryProps = ObjectProperties{
	TableName:           "project__repository",
	ForeignColumnSuffix: "repository_id",
	PrimaryColumnName:   "id",
	Type:                reflect.TypeOf(Repository{}),
}

var TemplateProps = ObjectProperties{
	TableName:         "project__template",
	SortableColumns:   []string{"name"},
	PrimaryColumnName: "id",
	Type:                reflect.TypeOf(Template{}),
}

var ScheduleProps = ObjectProperties{
	TableName:         "project__schedule",
	PrimaryColumnName: "id",
	Type:              reflect.TypeOf(Schedule{}),
}

var ProjectUserProps = ObjectProperties{
	TableName:         "project__user",
	PrimaryColumnName: "user_id",
	Type:              reflect.TypeOf(ProjectUser{}),
}

var ProjectProps = ObjectProperties{
	TableName:         "project",
	IsGlobal:          true,
	PrimaryColumnName: "id",
	Type:              reflect.TypeOf(Project{}),
}

var UserProps = ObjectProperties{
	TableName:         "user",
	IsGlobal:          true,
	PrimaryColumnName: "id",
	Type:              reflect.TypeOf(User{}),
}

var SessionProps = ObjectProperties{
	TableName:         "session",
	PrimaryColumnName: "id",
	Type:              reflect.TypeOf(Session{}),
}

var TokenProps = ObjectProperties{
	TableName:         "user__token",
	PrimaryColumnName: "id",
}

var TaskProps = ObjectProperties{
	TableName:         "task",
	IsGlobal:          true,
	PrimaryColumnName: "id",
	SortInverted:      true,
	Type:              reflect.TypeOf(Task{}),
}

var TaskOutputProps = ObjectProperties{
	TableName: "task__output",
	Type:      reflect.TypeOf(TaskOutput{}),
}
