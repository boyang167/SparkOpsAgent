package http

import "github.com/gin-gonic/gin"

const requirementDeprecationSunset = "2026-12-31"
const requirementSuccessorLink = `</api/v1/products/{productId}/release-plans>; rel="successor-version"`

func markRequirementDeprecated(c *gin.Context) {
	c.Header("Deprecation", "true")
	c.Header("Sunset", requirementDeprecationSunset)
	c.Header("Link", requirementSuccessorLink)
	c.Header("X-SparkOps-Deprecated-Reason", "ProductRequirement API is replaced by ReleasePlan; see /release-plans")
}
