package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	bolt "go.etcd.io/bbolt"
	"gopkg.in/yaml.v2"
)

// getBuildConfig returns job instance of already executed build
func getBuildConfig(buildID int) (*Job, error) {
	// Collect tasks info by reconstructing job object
	buildConfigDir := Config.WorkDir + "wakespace/" + strconv.Itoa(buildID)
	newConfigFilename := buildConfigDir + "/build_plan" + Config.jobsExt
	oldConfigFilename := buildConfigDir + "/build" + Config.jobsExt

	job := &Job{}

	// Check if a config file with new format exists
	_, err := os.Stat(newConfigFilename)
	if os.IsNotExist(err) {
		// Check if a config file with old format exists
		_, err = os.Stat(oldConfigFilename)
		if os.IsNotExist(err) {
			return nil, err
		}

		job, err = CreateJobFromFile(oldConfigFilename)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		// Config file with new format might exist but an error occured
		return nil, err
	} else {
		// Config file with new format exists
		data, err := os.ReadFile(newConfigFilename)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(data, job)
		if err != nil {
			return nil, err
		}
	}
	return job, nil
}

// HandleGetBuild Returns information required to bootstrap build page
// @Summary      Return status of the build
// @Description  Contains information about the job and the latest build status
// @Tags         build
// @Produce      json
// @Param        id       path    integer   true  "Build ID"
// @Success      200      {object}   GetBuildPayload
// @Failure      500      {string}   http.StatusInternalServerError
// @Failure      404      {string}   http.StatusNotFound
// @Router       /build/{id} [get]
func HandleGetBuild(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}

	idp := chi.URLParam(r, "id")
	buildID, err := strconv.Atoi(idp)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}

	job, err := getBuildConfig(buildID)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}

	// Get build statusupdate
	var buildStatusData BuildUpdateData
	err = DB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(HistoryBucket))
		ud := b.Get(Itob(buildID))
		if ud == nil {
			return fmt.Errorf("not found")
		}
		return json.Unmarshal(ud, &buildStatusData)
	})
	if err != nil {
		logger.Println(err)
		if err.Error() == "Not found" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(err.Error()))
		}
		return
	}
	payload := GetBuildPayload{
		Job:          job,
		StatusUpdate: &buildStatusData,
	}

	payloadB, err := json.Marshal(payload)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(payloadB)
}

type GetBuildPayload struct {
	Job          *Job             `json:"job"`
	StatusUpdate *BuildUpdateData `json:"status_update"`
}

// HandleAbortBuild aborts build
// @Summary      Abort the build
// @Tags         build
// @Produce      plain
// @Param        id       path    integer   true  "Build ID"
// @Success      200      {string}   string
// @Failure      500      {string}   http.StatusInternalServerError
// @Failure      404      {string}   http.StatusNotFound
// @Router       /build/{id}/abort [post]
func HandleAbortBuild(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}
	buildID := chi.URLParam(r, "id")
	id, err := strconv.Atoi(buildID)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}
	err = GlobalQueue.Abort(id, StatusAborted)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

// HandleFlushTaskLogs signals to flush logs
// @Summary      Signal the build to flush its log buffer
// @Tags         build
// @Produce      plain
// @Param        id       path    integer   true  "Build ID"
// @Success      200      {string}   string
// @Failure      500      {string}   http.StatusInternalServerError
// @Failure      404      {string}   http.StatusNotFound
// @Router       /build/{id}/flush [post]
func HandleFlushTaskLogs(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}

	buildID := chi.URLParam(r, "id")
	id, err := strconv.Atoi(buildID)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}

	err = GlobalQueue.FlushLogs(id)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

// HandleStartBuild takes the build from the queue and starts to run it immediately
// @Summary      Takes the build from the queue and starts to run it immediately, ignorring the number of allowed concurrent builds and concurrency parameter
// @Tags         build
// @Produce      plain
// @Param        id       path    integer   true  "Build ID"
// @Success      200      {string}   string
// @Failure      500      {string}   http.StatusInternalServerError
// @Failure      404      {string}   http.StatusNotFound
// @Router       /build/{id}/abort [post]
func HandleStartBuild(w http.ResponseWriter, r *http.Request) {
	logger, ok := r.Context().Value(HL).(*log.Logger)
	if !ok {
		logger = Logger
	}
	buildID := chi.URLParam(r, "id")
	id, err := strconv.Atoi(buildID)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(err.Error()))
		return
	}
	err = GlobalQueue.TakeNow(id)
	if err != nil {
		logger.Println(err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
}
