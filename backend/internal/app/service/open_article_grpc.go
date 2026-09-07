package service

import (
	"context"
	"errors"

	blogopenv1 "codeup.aliyun.com/qimao/blog/ai-blog/backend/api/blogopen/v1"
	articledomain "codeup.aliyun.com/qimao/blog/ai-blog/backend/internal/domain/article"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OpenArticleGRPCService 将开放文章协议适配到文章上下文公开查询接口。
type OpenArticleGRPCService struct {
	blogopenv1.UnimplementedArticleServiceServer                                // UnimplementedArticleServiceServer 保持生成接口的前向兼容。
	articles                                     articledomain.OpenQueryUseCase // articles 是文章上下文公开的只读应用接口。
}

// NewOpenArticleGRPCServer 创建开放文章查询 gRPC 服务。
func NewOpenArticleGRPCServer(articles articledomain.OpenQueryUseCase) blogopenv1.ArticleServiceServer {
	// 1. 启动阶段拒绝缺少文章公开查询接口
	if articles == nil {
		panic("开放文章 gRPC 服务缺少查询接口")
	}
	return &OpenArticleGRPCService{articles: articles}
}

// GetAvailableList 使用 Offset 分页返回已发表文章。
func (s *OpenArticleGRPCService) GetAvailableList(ctx context.Context, request *blogopenv1.GetAvailableListRequest) (*blogopenv1.GetAvailableListReply, error) {
	// 1. 在调用文章上下文前校验开放协议分页边界
	if request == nil || request.GetPage() == 0 || request.GetPageSize() == 0 || request.GetPageSize() > 100 {
		return nil, status.Error(codes.InvalidArgument, "page 必须大于 0，page_size 必须位于 1 至 100")
	}

	// 2. 只通过文章上下文公开接口查询已发表文章
	result, err := s.articles.ListAvailable(ctx, articledomain.AvailableListCommand{Page: request.GetPage(), PageSize: request.GetPageSize(), IsDesc: request.GetIsDesc()})
	if err != nil {
		return nil, articleGRPCError(err)
	}
	if result == nil {
		return nil, status.Error(codes.Internal, "查询已发表文章失败")
	}

	// 3. 转换开放字段，不泄漏正文、作者或互动投影
	reply := &blogopenv1.GetAvailableListReply{Items: make([]*blogopenv1.AvailableArticle, 0, len(result.Articles)), Total: result.Total, Page: result.Page, PageSize: result.PageSize}
	for _, current := range result.Articles {
		if current == nil {
			continue
		}
		reply.Items = append(reply.Items, &blogopenv1.AvailableArticle{Id: current.ID, Title: current.Title, Tags: append([]string(nil), current.Tags...), CreatedTime: unixSeconds(current.CreatedTime), UpdatedTime: unixSeconds(current.UpdatedTime)})
	}
	return reply, nil
}

// articleGRPCError 将文章查询错误映射为标准 gRPC Code。
func articleGRPCError(err error) error {
	// 1. 分页错误映射为 InvalidArgument，其他依赖错误隐藏为 Internal
	if errors.Is(err, articledomain.ErrInvalidPagination) {
		return status.Error(codes.InvalidArgument, "文章分页参数不合法")
	}
	return status.Error(codes.Internal, "查询已发表文章失败")
}
