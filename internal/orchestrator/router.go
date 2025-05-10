package orchestrator

import "github.com/gin-gonic/gin"

func SetupRouter(o *Orchestrator, auth *AuthHandler) *gin.Engine {
	router := gin.Default()

	apiV1 := router.Group("/api/v1")
	{
		apiV1.POST("/register", auth.RegisterHandler)
		apiV1.POST("/login", auth.LoginHandler)

		authorized := apiV1.Group("/")
		authorized.Use(auth.AuthorizationMiddleware())
		{
			authorized.POST("/calculate", o.CalculateHandler)
		}
	}
	return router
}
