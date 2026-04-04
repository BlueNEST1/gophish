package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	ctx "github.com/gophish/gophish/context"
	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
)

// Campaigns returns a list of campaigns if requested via GET.
// If requested via POST, APICampaigns creates a new campaign and returns a reference to it.
func (as *Server) Campaigns(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaigns(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, cs, http.StatusOK)
	//POST: Create a new campaign and return it as JSON
	case r.Method == "POST":
		c := models.Campaign{}
		// Put the request into a campaign
		err := json.NewDecoder(r.Body).Decode(&c)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		err = models.PostCampaign(&c, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// If the campaign is scheduled to launch immediately, send it to the worker.
		// Otherwise, the worker will pick it up at the scheduled time
		if c.Status == models.CampaignInProgress {
			go as.worker.LaunchCampaign(c)
		}
		JSONResponse(w, c, http.StatusCreated)
	}
}

// CampaignsSummary returns the summary for the current user's campaigns
func (as *Server) CampaignsSummary(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummaries(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// Campaign returns details about the requested campaign. If the campaign is not
// valid, APICampaign returns null.
func (as *Server) Campaign(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	c, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, c, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteCampaign(id)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign deleted successfully!"}, http.StatusOK)
	}
}

// CampaignResults returns just the results for a given campaign to
// significantly reduce the information returned.
func (as *Server) CampaignResults(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	cr, err := models.GetCampaignResults(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		JSONResponse(w, cr, http.StatusOK)
		return
	}
}

// CampaignSummary returns the summary for a given campaign.
func (as *Server) CampaignSummary(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		cs, err := models.GetCampaignSummary(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
			} else {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			}
			log.Error(err)
			return
		}
		JSONResponse(w, cs, http.StatusOK)
	}
}

// CampaignAnalysis returns a derived per-user summary of campaign events,
// with one record per target showing timestamps and counts for each event type.
func (as *Server) CampaignAnalysis(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	_, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		records, err := models.GetCampaignAnalysis(id)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, records, http.StatusOK)
	}
}

// CampaignCompare returns a side-by-side metric comparison between two campaigns.
func (as *Server) CampaignCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	idA, err := strconv.ParseInt(r.URL.Query().Get("campaign_a"), 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_a"}, http.StatusBadRequest)
		return
	}
	idB, err := strconv.ParseInt(r.URL.Query().Get("campaign_b"), 0, 64)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid campaign_b"}, http.StatusBadRequest)
		return
	}
	cA, err := models.GetCampaign(idA, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign A not found"}, http.StatusNotFound)
		return
	}
	cB, err := models.GetCampaign(idB, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign B not found"}, http.StatusNotFound)
		return
	}
	mA, err := models.GetCampaignMetrics(idA)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	mB, err := models.GetCampaignMetrics(idB)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	var timeDiff *float64
	if mA.AverageTimeToClickSeconds != nil && mB.AverageTimeToClickSeconds != nil {
		d := *mB.AverageTimeToClickSeconds - *mA.AverageTimeToClickSeconds
		timeDiff = &d
	}
	result := models.CampaignComparisonResult{
		CampaignA: models.CampaignComparisonEntry{
			Id:                        idA,
			Name:                      cA.Name,
			UnsafeInteractionRate:     mA.UnsafeInteractionRate,
			SubmissionRate:            mA.SubmissionRate,
			AverageTimeToClickSeconds: mA.AverageTimeToClickSeconds,
		},
		CampaignB: models.CampaignComparisonEntry{
			Id:                        idB,
			Name:                      cB.Name,
			UnsafeInteractionRate:     mB.UnsafeInteractionRate,
			SubmissionRate:            mB.SubmissionRate,
			AverageTimeToClickSeconds: mB.AverageTimeToClickSeconds,
		},
		Difference: models.CampaignMetricsDiff{
			UnsafeInteractionRate:     mB.UnsafeInteractionRate - mA.UnsafeInteractionRate,
			SubmissionRate:            mB.SubmissionRate - mA.SubmissionRate,
			AverageTimeToClickSeconds: timeDiff,
		},
	}
	JSONResponse(w, result, http.StatusOK)
}

// CampaignMetrics returns computed behavioural metrics for a campaign.
func (as *Server) CampaignMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	_, err := models.GetCampaign(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Campaign not found"}, http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		m, err := models.GetCampaignMetrics(id)
		if err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, m, http.StatusOK)
	}
}

// CampaignComplete effectively "ends" a campaign.
// Future phishing emails clicked will return a simple "404" page.
func (as *Server) CampaignComplete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	switch {
	case r.Method == "GET":
		err := models.CompleteCampaign(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error completing campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Campaign completed successfully!"}, http.StatusOK)
	}
}
