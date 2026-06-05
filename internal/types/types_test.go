package types

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestTestResultValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		res     TestResult
		wantErr bool
	}{
		{
			name: "Valid Result",
			res: TestResult{
				TestName:   "TestFoo",
				Status:     "pass",
				DurationMS: 100,
			},
			wantErr: false,
		},
		{
			name: "Missing TestName",
			res: TestResult{
				Status:     "pass",
				DurationMS: 100,
			},
			wantErr: true,
		},
		{
			name: "Invalid Status",
			res: TestResult{
				TestName:   "TestFoo",
				Status:     "running",
				DurationMS: 100,
			},
			wantErr: true,
		},
		{
			name: "Negative Duration",
			res: TestResult{
				TestName:   "TestFoo",
				Status:     "pass",
				DurationMS: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.res)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTestEventValidation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		event   TestEvent
		wantErr bool
	}{
		{
			name: "Valid Event",
			event: TestEvent{
				Action:  "pass",
				Elapsed: 0.5,
			},
			wantErr: false,
		},
		{
			name: "Missing Action",
			event: TestEvent{
				Elapsed: 0.5,
			},
			wantErr: true,
		},
		{
			name: "Invalid Action",
			event: TestEvent{
				Action:  "invalid",
				Elapsed: 0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("validate.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
