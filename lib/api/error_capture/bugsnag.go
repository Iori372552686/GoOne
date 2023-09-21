package error_capture

import (
	"context"
	"fmt"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/bugsnag/bugsnag-go/v2/errors"
)

const (
	api_key_beta    = "7c32449d85562958d7a889ffa647a6be"
	api_key_ready   = "fa3c75ab45f045eb718f8d9488937605"
	api_key_release = "2b8cb6ea1cf7bb03e72bda5071617b93"
)

type BugsnagVersion string

const (
	BUGSNAG_VERSION_BETA    BugsnagVersion = "beta"
	BUGSNAG_VERSION_READY   BugsnagVersion = "ready"
	BUGSNAG_VERSION_RELEASE BugsnagVersion = "release"
)

type bugsnagInfo struct {
	using          bool
	bugsnagVersion BugsnagVersion
	apiKey         string
	serviceName    string
	playId         string
	metaData       bugsnag.MetaData
}

var info = bugsnagInfo{}

func initBugsnagInfo(bugsnagVersion BugsnagVersion, serviceName, playId string) {
	info.using = true
	info.bugsnagVersion = bugsnagVersion
	info.serviceName = serviceName
	info.playId = playId
	info.metaData = bugsnag.MetaData{
		"Account": {
			"Name":   serviceName,
			"Paying": playId,
		},
	}

	switch bugsnagVersion {
	case BUGSNAG_VERSION_READY:
		info.apiKey = api_key_ready
	case BUGSNAG_VERSION_RELEASE:
		info.apiKey = api_key_release
	default:
		info.apiKey = api_key_beta
	}
}

func InitBugsnag(bugsnagVersion BugsnagVersion, serviceName, playId string) {
	initBugsnagInfo(bugsnagVersion, serviceName, playId)
	bugsnag.Configure(bugsnag.Configuration{
		// Your Bugsnag project API key, required unless set as environment variable $BUGSNAG_API_KEY
		APIKey: info.apiKey,
		//APIKey: "98df5c261ca42798a7d383de7e2a6e59",

		// The development stage of your application build, like "alpha" or "production"
		ReleaseStage: info.serviceName,

		// The import paths for the Go packages containing your source files
		ProjectPackages: []string{"main", "github.com/org/myapp"},

		// more configuration options
	})
	bugsnag.StartSession(context.Background())
}

func Error(userId, userName, f string, v ...interface{}) {
	if !info.using {
		return
	}

	// // send an error object directly
	// // bugsnag.Notify(err,ctx)
	//
	// // construct an Error with a format string
	// // bugsnag.Notify(errors.Errorf("broken pipe: %d", code))
	//
	// // wrap an error object, trimming 0 stack frames (more info below)
	// // bugsnag.Notify(errors.New(err, 0))
	if userId == "" {
		userId = info.playId
	}

	ctx := bugsnag.StartSession(context.Background())
	bugsnag.Notify(
		errors.New(fmt.Errorf(f, v...), 2),
		ctx,
		bugsnag.MetaData{
			"Account": {
				"Name":   info.serviceName,
				"Paying": info.playId,
			},
		},
		bugsnag.HandledState{
			SeverityReason:   bugsnag.SeverityReasonUnhandledError,
			OriginalSeverity: bugsnag.SeverityError,
			Unhandled:        false,
		},
		bugsnag.User{
			Id:   userId,
			Name: userName,
		},
		bugsnag.ErrorClass{Name: f},
	)
}
