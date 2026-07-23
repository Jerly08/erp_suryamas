package controllers

import (
	"app-sistem-akuntansi/models"
	"app-sistem-akuntansi/services"
	"app-sistem-akuntansi/utils"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProjectController struct {
	service services.ProjectService
}

func NewProjectController(service services.ProjectService) *ProjectController {
	return &ProjectController{service: service}
}

func (c *ProjectController) GetAll(ctx *gin.Context) {
	hasPaginationQuery := ctx.Query("page") != "" || ctx.Query("limit") != "" || ctx.Query("search") != ""
	if hasPaginationQuery {
		page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "10"))
		if page < 1 {
			page = 1
		}
		if limit < 1 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		projects, total, err := c.service.GetProjects(models.ProjectFilter{
			Page:   page,
			Limit:  limit,
			Search: strings.TrimSpace(ctx.Query("search")),
		})
		if err != nil {
			c.respondError(ctx, err, "Failed to load projects")
			return
		}

		totalPages := int(math.Ceil(float64(total) / float64(limit)))
		if totalPages < 1 {
			totalPages = 1
		}
		if page > totalPages {
			page = totalPages
		}

		ctx.JSON(http.StatusOK, gin.H{
			"data": projects,
			"meta": gin.H{
				"pagination": gin.H{
					"page":        page,
					"limit":       limit,
					"total":       total,
					"total_pages": totalPages,
				},
			},
		})
		return
	}

	projects, err := c.service.GetAllProjects()
	if err != nil {
		c.respondError(ctx, err, "Failed to load projects")
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": projects})
}

func (c *ProjectController) GetByID(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	project, err := c.service.GetProjectByID(uint(id))
	if err != nil {
		c.respondError(ctx, err, "Failed to load project")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": project})
}

func (c *ProjectController) Create(ctx *gin.Context) {
	var req models.ProjectCreateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := c.service.CreateProject(req)
	if err != nil {
		c.respondError(ctx, err, "Failed to create project")
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"data": project})
}

func (c *ProjectController) Update(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	var req models.ProjectUpdateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := c.service.UpdateProject(uint(id), req)
	if err != nil {
		c.respondError(ctx, err, "Failed to update project")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": project})
}

func (c *ProjectController) Delete(ctx *gin.Context) {
	idStr := ctx.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID format"})
		return
	}

	if err := c.service.DeleteProject(uint(id)); err != nil {
		c.respondError(ctx, err, "Failed to delete project")
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Project deleted successfully"})
}

func (c *ProjectController) respondError(ctx *gin.Context, err error, fallbackMessage string) {
	if appErr := utils.GetAppError(err); appErr != nil {
		ctx.JSON(appErr.StatusCode, appErr.ToErrorResponse(""))
		return
	}

	internalErr := utils.NewInternalError(fallbackMessage, err)
	ctx.JSON(internalErr.StatusCode, internalErr.ToErrorResponse(""))
}
