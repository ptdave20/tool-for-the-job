package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func PostTodo(c *gin.Context) {
	var input Todo
	if err := c.ShouldBindBodyWithJSON(&input); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, err)
		return
	}

	dbCon, err := GetPostgresConn(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}

	result, err := dbCon.Exec(c.Request.Context(), "insert into public.todos (title, done) values ($1, $2)", input.Title, input.Done)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		c.Error(err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, "No rows affected")
	}

	c.Status(http.StatusAccepted)
}

func GetTodo(c *gin.Context) {
	dbCon, err := GetPostgresConn(c)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	rows, err := dbCon.Query(c.Request.Context(), "select id, title, done from public.todos;")
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		return
	}
	defer rows.Close()

	var ret []Todo

	for rows.Next() {
		if err = rows.Err(); err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, err)
		}
		var row Todo
		err := rows.Scan(&row.Id, &row.Title, &row.Done)
		if err != nil {
			c.AbortWithError(500, err)
			break
		}
		ret = append(ret, row)
	}
	c.JSON(200, ret)
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

	dbCon, err := GetPostgresConn(c)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	result, err := dbCon.Exec(c.Request.Context(), "update public.todos set title = $1, done = $2 where id = $3", input.Title, input.Done, input.Id)
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
