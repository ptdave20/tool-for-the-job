package main

import (
	_ "embed"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

//go:embed queries/get_todos_paginated.sql
var getTodosPaginatedQuery string

func PostTodo(c *gin.Context) {
	var input Todo
	if err := c.ShouldBindBodyWithJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}

	dbPool, err := GetPostgresConn(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	err = dbPool.QueryRow(c.Request.Context(), "insert into public.todos (title, done) values ($1, $2) RETURNING id;", input.Title, input.Done).Scan(&input.Id)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"id": input.Id})
}

func GetTodo(c *gin.Context) {
	// Parse query parameters
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, Paginated[Todo]{
			Message: "Invalid page number",
		})
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		c.JSON(http.StatusBadRequest, Paginated[Todo]{
			Message: "Invalid page size (1-100)",
		})
		return
	}

	dbPool, err := GetPostgresConn(c)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Calculate offset
	offset := (page - 1) * pageSize
	var totalCount int
	var items []Todo

	rows, err := dbPool.Query(c.Request.Context(),
		getTodosPaginatedQuery, pageSize, offset)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var todo Todo
		err := rows.Scan(&todo.Id, &todo.Title, &todo.Done, &totalCount)
		if err != nil {
			c.JSON(http.StatusInternalServerError, Paginated[Todo]{
				Message: "Error scanning row",
			})
			return
		}
		items = append(items, todo)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, Paginated[Todo]{
			Message: "Error scanning row",
		})
		return
	}

	c.JSON(http.StatusOK, Paginated[Todo]{
		Data:  items,
		Total: totalCount,
		Size:  pageSize,
		Page:  page,
	})
}

func PatchTodo(c *gin.Context) {
	var input Todo
	var err error
	if err := c.ShouldBindBodyWithJSON(&input); err != nil {
		c.AbortWithError(400, err)
		return
	}

	idKey := c.Param("id")
	input.Id, err = strconv.Atoi(idKey)
	if err != nil {
		c.AbortWithError(400, err)
		return
	}

	dbPool, err := GetPostgresConn(c)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	result, err := dbPool.Exec(c.Request.Context(), "update public.todos set title = $1, done = $2 where id = $3", input.Title, input.Done, input.Id)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	if result.RowsAffected() == 0 {
		c.AbortWithError(http.StatusNotFound, errors.New("No rows affected"))
		return
	}
	c.Status(http.StatusAccepted)
}
