package controllers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/imyousuf/webhook-broker/config"
	"github.com/imyousuf/webhook-broker/storage"
	"github.com/imyousuf/webhook-broker/storage/data"
	storagemocks "github.com/imyousuf/webhook-broker/storage/mocks"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

var (
	configuration *config.Config
	seedData      *config.SeedData
	defaultApp    *data.App
	db            *sql.DB
)

func TestMain(m *testing.M) {
	var err error
	configuration, err = config.GetConfiguration("./controller-test-config.cfg")
	if err == nil {
		seedData = &configuration.SeedData
		defaultApp = data.NewApp(seedData, data.Initialized)
		migrationLocation, _ := filepath.Abs("../migration/sqls/")
		os.Remove("./webhook-broker.sqlite3")
		db, err = storage.GetConnectionPool(configuration, &storage.MigrationConfig{MigrationEnabled: true, MigrationSource: "file://" + migrationLocation}, configuration)
		if err == nil {
			ProducerTestSetup()
			ChannelTestSetup()
			m.Run()
			db.Close()
		}
	}
	if err != nil {
		log.Fatalln(err)
	}
}

func setupTestRoutes(r *httprouter.Router, s EndpointController) {
	setupAPIRoutes(r, s)
}

func TestStatus(t *testing.T) {
	mAppRepo := new(storagemocks.AppRepository)
	testRouter := httprouter.New()
	statusController := NewStatusController(mAppRepo)
	setupTestRoutes(testRouter, statusController)
	mAppRepo.On("GetApp").Return(defaultApp, nil)
	req, _ := http.NewRequest("GET", "/_status", nil)
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	outAppData := &AppData{}
	body := rr.Body.String()
	t.Log(body)
	json.NewDecoder(strings.NewReader(body)).Decode(outAppData)
	assert.Equal(t, AppData{SeedData: defaultApp.GetSeedData(), AppStatus: defaultApp.GetStatus()}, *outAppData)
	mAppRepo.AssertExpectations(t)
	assert.Equal(t, statusPath, statusController.FormatAsRelativeLink())
}

func TestStatus_AppDataError(t *testing.T) {
	mAppRepo := new(storagemocks.AppRepository)
	testRouter := httprouter.New()
	setupTestRoutes(testRouter, NewStatusController(mAppRepo))
	err := errors.New("App could not be returned")
	mAppRepo.On("GetApp").Return(defaultApp, err)
	req, _ := http.NewRequest("GET", "/_status", nil)
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, err.Error(), rr.Body.String())
	mAppRepo.AssertExpectations(t)
}

func TestStatus_JSONMarshalError(t *testing.T) {
	mAppRepo := new(storagemocks.AppRepository)
	testRouter := httprouter.New()
	setupTestRoutes(testRouter, NewStatusController(mAppRepo))
	mAppRepo.On("GetApp").Return(defaultApp, nil)
	err := errors.New("App could not be returned")
	oldGetJSON := getJSON
	getJSON = func(buf *bytes.Buffer, app interface{}) error {
		return err
	}
	defer func() {
		getJSON = oldGetJSON
	}()
	req, _ := http.NewRequest("GET", "/_status", nil)
	rr := httptest.NewRecorder()
	testRouter.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Equal(t, err.Error(), rr.Body.String())
	mAppRepo.AssertExpectations(t)
}
