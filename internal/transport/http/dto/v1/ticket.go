// Package v1 — версионированный контракт ответов v1 API. Реализация целиком
// делегирована уже существующему пакету dto (он появился до введения
// версионирования и де-факто всегда был контрактом v1) — типы и функции
// здесь просто алиасы, чтобы v1-handler'ы могли импортировать
// "dto/v1" явно, как и v2, но без риска расхождения в поведении и без
// дублирования ~200 строк логики маппинга.
package v1

import "pet-ticket/internal/transport/http/dto"

type (
	TicketCreateRequest        = dto.TicketCreateRequest
	TicketUpdateRequest        = dto.TicketUpdateRequest
	TicketResponse             = dto.TicketResponse
	SLAMetricsResponse         = dto.SLAMetricsResponse
	TicketWithSLAResponse      = dto.TicketWithSLAResponse
	TicketHistoryResponse      = dto.TicketHistoryResponse
	StatusResponse             = dto.StatusResponse
	TopicResponse              = dto.TopicResponse
	ListResponse               = dto.ListResponse
	PaginationResponse         = dto.PaginationResponse
	ListResponseWithPagination = dto.ListResponseWithPagination
	ErrorResponse              = dto.ErrorResponse
	ErrorDetail                = dto.ErrorDetail
)

var (
	ToTicketResponse            = dto.ToTicketResponse
	ToTicketWithSLAResponse     = dto.ToTicketWithSLAResponse
	ToTicketWithSLAResponseList = dto.ToTicketWithSLAResponseList
	ToTicketHistoryResponse     = dto.ToTicketHistoryResponse
	ToTicketHistoryResponseList = dto.ToTicketHistoryResponseList
	ToTicketResponseList        = dto.ToTicketResponseList
	ToStatusResponse            = dto.ToStatusResponse
	ToStatusResponseList        = dto.ToStatusResponseList
	ToTopicResponse             = dto.ToTopicResponse
	ToTopicResponseList         = dto.ToTopicResponseList
)
