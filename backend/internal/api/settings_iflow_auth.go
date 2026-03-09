package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type startIFLOWBrowserAuthRequest struct {
	CommandPath string `json:"command_path,omitempty"`
}

type submitIFLOWBrowserAuthCodeRequest struct {
	AuthorizationCode string `json:"authorization_code"`
}

func startIFLOWBrowserAuthHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.IFLOWAuth == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "iflow auth manager not configured"})
			return
		}
		var req startIFLOWBrowserAuthRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		session, err := deps.IFLOWAuth.Start(strings.TrimSpace(req.CommandPath))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}

func getIFLOWBrowserAuthHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.IFLOWAuth == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "iflow auth manager not configured"})
			return
		}
		session, err := deps.IFLOWAuth.Get(c.Param("id"))
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "iflow auth session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}

func submitIFLOWBrowserAuthCodeHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.IFLOWAuth == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "iflow auth manager not configured"})
			return
		}
		var req submitIFLOWBrowserAuthCodeRequest
		if b, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)); len(b) > 0 {
			if err := json.Unmarshal(b, &req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
		}
		session, err := deps.IFLOWAuth.SubmitCode(c.Param("id"), req.AuthorizationCode)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "iflow auth session not found"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}

func cancelIFLOWBrowserAuthHandler(deps Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		if deps.IFLOWAuth == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "iflow auth manager not configured"})
			return
		}
		session, err := deps.IFLOWAuth.Cancel(c.Param("id"))
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "iflow auth session not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, session)
	}
}
