package utils

// 统一响应壳 { code, msg, data }。约定:
//   - HTTP 状态码为真实状态(成功 200/201,失败 4xx/5xx),便于 axios 拦截器 / 监控 / 网关识别。
//   - body 里的 code 与 HTTP 状态一致,仅作前端可读的冗余;msg 给人看;data 仅成功时带。

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// Success 200 + 数据
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: http.StatusOK, Msg: "success", Data: data})
}

// Created 201 + 数据(资源创建)
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Code: http.StatusCreated, Msg: "success", Data: data})
}

// SuccessMsg 200 + 仅消息(无数据体,如删除)
func SuccessMsg(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, Response{Code: http.StatusOK, Msg: msg})
}

// Error 以真实 HTTP 状态返回错误;code 与 HTTP 状态一致。
func Error(c *gin.Context, code int, msg string) {
	c.JSON(code, Response{Code: code, Msg: msg})
}

func BadRequest(c *gin.Context, msg string)      { Error(c, http.StatusBadRequest, msg) }
func Unauthorized(c *gin.Context, msg string)    { Error(c, http.StatusUnauthorized, msg) }
func Forbidden(c *gin.Context, msg string)       { Error(c, http.StatusForbidden, msg) }
func NotFound(c *gin.Context, msg string)        { Error(c, http.StatusNotFound, msg) }
func Conflict(c *gin.Context, msg string)        { Error(c, http.StatusConflict, msg) }
func TooManyRequests(c *gin.Context, msg string) { Error(c, http.StatusTooManyRequests, msg) }
func ServerError(c *gin.Context, msg string)     { Error(c, http.StatusInternalServerError, msg) }

// AbortUnauthorized / AbortForbidden 供中间件用:写响应并终止后续 handler。
func AbortUnauthorized(c *gin.Context, msg string) { Unauthorized(c, msg); c.Abort() }
func AbortForbidden(c *gin.Context, msg string)    { Forbidden(c, msg); c.Abort() }
func AbortServerError(c *gin.Context, msg string)  { ServerError(c, msg); c.Abort() }
