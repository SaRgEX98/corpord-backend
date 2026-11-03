package middleware

import "github.com/gin-gonic/gin"

// Legacy arrays remain usable for small clients but never serialize a national
// timetable. New clients follow the cursor-based successor endpoint.
func CollectionLimits() gin.HandlerFunc {
	kinds := map[string]string{
		"/api/v1/bus/": "buses", "/api/v1/driver/": "drivers", "/api/v1/stops/": "stops",
		"/api/v1/trips/": "trips", "/api/v1/trips/all": "trips", "/api/v1/trip_stops/": "tripStops",
		"/api/v1/bus/categories/": "busCategories", "/api/v1/bus/statuses/": "busStatuses", "/api/v1/driver/status/": "driverStatuses",
		"/api/v1/admin/cities": "cities", "/api/v1/admin/users": "users", "/api/v1/users": "users",
	}
	return func(c *gin.Context) {
		if c.Request.Method == "GET" {
			if kind, ok := kinds[c.Request.URL.Path]; ok {
				path := "/api/v1/catalog/" + kind
				if kind == "users" {
					path = "/api/v1/admin/catalog/users"
				}
				c.Header("X-Collection-Limit", "100")
				c.Header("Link", "<"+path+">; rel=\"successor-version\"")
			}
		}
		c.Next()
	}
}
