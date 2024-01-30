package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jbrit/gopaypeer/models"
	"gorm.io/gorm"
)

func requireAuth(c *gin.Context, db *gorm.DB) (*models.User, error) {
	claims := &Claims{}
	if err := VerifyJwt(c.GetHeader("Authorization"), jwtKey, claims); err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return nil, err
	}

	var user models.User
	if err := db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication token"})
		return nil, fmt.Errorf("")
	}
	return &user, nil
}

func CurrentUser(c *gin.Context, db *gorm.DB) {

	user, err := requireAuth(c, db)
	if err != nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user})
}

func PubkeyToUser(c *gin.Context, db *gorm.DB) {
	var users []models.User
	// Get all records
	result := db.Find(&users)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch addresses"})
	}

	pubkeyToUser := map[string]string{}
	for _, user := range users {
		pubkeyToUser[user.PublicKey] = user.Email

	}

	c.JSON(http.StatusOK, pubkeyToUser)
}
