package testing

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/mock"
)

const (
	taskOneID = ""
)

// TaskFields will include the IDGenerator, and Tasks
type TaskFields struct {
	IDGenerator   influxdb.IDGenerator
	OrgTaskIDs    influxdb.IDGenerator
	TimeGenerator influxdb.TimeGenerator
	Tasks         []*influxdb.Task
	Organizations []*influxdb.Organization
}

type taskServiceF func(
	init func(TaskFields, *testing.T) (influxdb.TaskService, string, func()),
	t *testing.T,
)

// TaskService tests all the service functions.
func TaskService(
	init func(TaskFields, *testing.T) (influxdb.TaskService, string, func()),
	t *testing.T,
) {
	tests := []struct {
		name string
		fn   taskServiceF
	}{
		{
			name: "CreateTask",
			fn:   CreateTask,
		},
		// {
		// 	name: "FindTasks",
		// 	fn:   FindTasks,
		// },
		// {
		// 	name: "FindTaskByID",
		// 	fn:   FindTaskByID,
		// },
		// {
		// 	name: "UpdateTask",
		// 	fn:   UpdateTask,
		// },
		// {
		// 	name: "DeleteTask",
		// 	fn:   DeleteTask,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fn(init, t)
		})
	}
}

// CreateTask testing
func CreateTask(
	init func(TaskFields, *testing.T) (influxdb.TaskService, string, func()),
	t *testing.T,
) {
	type args struct {
		task influxdb.TaskCreate
	}
	type wants struct {
		err   error
		tasks []*influxdb.Task
	}

	tests := []struct {
		name   string
		fields TaskFields
		args   args
		wants  wants
	}{
		{
			name: "Create a basic task",
			fields: TaskFields{
				IDGenerator: &mock.IDGenerator{
					IDFn: func() influxdb.ID {
						return MustIDBase16(taskOneID)
					},
				},
				TimeGenerator: mock.TimeGenerator{FakeValue: time.Date(2009, time.November, 10, 24, 0, 0, 0, time.UTC)},
				Tasks:         []*influxdb.Task{},
			},
			args: args{
				task: influxdb.TaskCreate{
					// ID:             MustIDBase16(taskOneID),
					OrganizationID: 1,
					Flux: `option task = {
name: "itty bitty task",
every: 10m,
}
from(bucket:"holder") |> range(start:-5m) |> to(bucket:"holder", org:"thing")`,
				},
			},
			wants: wants{
				tasks: []*influxdb.Task{
					{
						ID:             MustIDBase16(dashOneID),
						OrganizationID: 1,
						Name:           "itty bitty task",
						Every:          "10m",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, done := init(tt.fields, t)
			defer done()
			ctx := context.Background()
			task, err := s.CreateTask(ctx, tt.args.task)

			defer s.DeleteTask(ctx, task.ID)

			tasks, _, err := s.FindTasks(ctx, influxdb.TaskFilter{})
			if err != nil {
				t.Fatalf("failed to retrieve tasks: %v", err)
			}

			fmt.Println(task)
			fmt.Println(tasks)
		})
	}
}

// FindTasks testing
// func FindTasks(
// 	init func(TaskFields, *testing.T) (influxdb.TaskService, string, func()),
// 	t *testing.T,
// ) {
// 	type args struct {
// 		IDs            []*influxdb.ID
// 		organizationID *influxdb.ID
// 		filter         influxdb.TaskFilter
// 	}

// 	type wants struct {
// 		tasks []*influxdb.Task
// 		err   error
// 	}

// 	tests := []struct {
// 		name   string
// 		fields TaskFields
// 		args   args
// 		wants  wants
// 	}{

// 	}
// }
