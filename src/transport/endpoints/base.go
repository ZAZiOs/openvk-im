package endpoints

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	redis "ovk-im/src/repo/redis"
	"ovk-im/src/transport/broadcaster"
)

type Router struct {
	DB          *gorm.DB
	LPRepo      *redis.Repo
	Broadcaster *broadcaster.Broadcaster
}

type VKError struct {
	ErrorCode     int            `json:"error_code"`
	ErrorMsg      string         `json:"error_msg"`
	RequestParams []RequestParam `json:"request_params,omitempty"`
}

type RequestParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (r *Router) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Query("key")
		if key == "" {
			key = c.GetHeader("Authorization")
			key = strings.TrimPrefix(key, "Bearer ")
		}

		if key == "" {
			r.Reject(c, 5, "User authorization failed: no key passed")
			c.Abort()
			return
		}

		userID, err := r.LPRepo.GetUserIDBySession(c.Request.Context(), key)
		if err != nil {
			r.Reject(c, 5, "User authorization failed: invalid session")
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func (r *Router) Register(group *gin.RouterGroup) {
	protected := group.Group("/")
	protected.Use(r.AuthMiddleware())

	protected.Match([]string{"GET", "POST"}, "/:slug", r.BasicHandler)
}
func (r *Router) Reject(c *gin.Context, errorCode int, errorMsg string) {
	params := make([]RequestParam, 0)
	for k, v := range c.Request.URL.Query() {
		params = append(params, RequestParam{Key: k, Value: v[0]})
	}

	c.JSON(http.StatusOK, VKError{
		ErrorCode:     errorCode,
		ErrorMsg:      errorMsg,
		RequestParams: params,
	})
}

func (r *Router) BasicHandler(c *gin.Context) {
	if c.IsAborted() {
		return
	}

	slug := c.Param("slug")

	methods := map[string]gin.HandlerFunc{
		"messages.send": r.MessagesSend,
	}

	handler, exists := methods[slug]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "method_not_found",
			"slug":  slug,
		})
		return
	}

	handler(c)
}

var reAttachment = regexp.MustCompile(`^(photo|video|audio|doc|wall|market|poll|question)-?\d+_\d+$`)

func IsValidAttachments(attachmentStr string) bool {
	if attachmentStr == "" {
		return true
	}

	parts := strings.Split(attachmentStr, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if !reAttachment.MatchString(part) {
			return false
		}
	}
	return true
}
