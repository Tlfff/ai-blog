package service

import (
	"errors"

	searchapi "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/search"
	searchdomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/search"
	"codeup.aliyun.com/qimao/leo/lib/errassets"
	"github.com/gin-gonic/gin"
)

const (
	codeSearchInvalid     = 44070101 // codeSearchInvalid 表示搜索参数错误。
	codeSearchUnavailable = 44070102 // codeSearchUnavailable 表示搜索基础设施不可用。
)

// SearchService 将搜索 HTTP 协议转换为搜索领域调用。
type SearchService struct {
	useCase searchdomain.UseCase // useCase 提供搜索领域能力。
}

// NewSearchServer 创建搜索 HTTP 服务。
func NewSearchServer(useCase searchdomain.UseCase) searchapi.SearchServiceHTTPServerController {
	// 1. 启动阶段拒绝缺少搜索领域能力
	if useCase == nil {
		panic("搜索 HTTP 服务缺少领域服务")
	}
	return &SearchService{useCase: useCase}
}

// SearchArticles 搜索已发表文章并转换高亮和摘要。
func (s *SearchService) SearchArticles(ctx *gin.Context, request *searchapi.SearchArticlesRequest) (*searchapi.SearchArticlesReply, error) {
	// 1. 转换协议输入并调用搜索领域服务
	result, err := s.useCase.Search(ctx.Request.Context(), searchdomain.Query{Keyword: request.GetKeyword(), Page: request.GetPage(), PageSize: request.GetPageSize()})
	if err != nil {
		return nil, searchHTTPError(err)
	}

	// 2. 转换公开响应字段并保留 Meilisearch 高亮结果
	reply := &searchapi.SearchArticlesReply{Total: result.Total, Page: result.Page, PageSize: result.PageSize, List: make([]*searchapi.SearchArticleItem, 0, len(result.Items))}
	for _, item := range result.Items {
		reply.List = append(reply.List, &searchapi.SearchArticleItem{Id: item.ID, Title: item.Title, TitleHighlight: item.TitleHighlight, Summary: item.Summary, Tags: append([]string{}, item.Tags...)})
	}
	return reply, nil
}

// searchHTTPError 将搜索领域错误映射为稳定业务错误。
func searchHTTPError(err error) error {
	// 1. 将参数错误和基础设施错误映射为公开业务码
	switch {
	case errors.Is(err, searchdomain.ErrInvalidQuery):
		return errassets.NewError(codeSearchInvalid, "搜索参数不合法")
	case errors.Is(err, searchdomain.ErrUnavailable):
		return errassets.NewError(codeSearchUnavailable, "搜索服务不可用")
	default:
		return err
	}
}
