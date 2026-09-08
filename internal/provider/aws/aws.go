// Package aws AWS Provider：使用 aws-sdk-go-v2 拉取注册域名（Route 53 Domains）与托管 Zone（Route 53）。
package aws

import (
	"context"
	"fmt"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"

	"github.com/domhub-io/domhub/internal/provider"
)

func init() {
	provider.Register("aws", func(cred provider.Credential) (provider.DomainProvider, error) {
		region := cred.Region
		if region == "" {
			region = "us-east-1" // route53domains 仅在 us-east-1 提供
		}
		return &Provider{cred: cred, region: region}, nil
	})
	provider.RegisterDNS("aws", func(cred provider.Credential) (provider.DNSProvider, error) {
		region := cred.Region
		if region == "" {
			region = "us-east-1"
		}
		return &Provider{cred: cred, region: region}, nil
	})
}

// Provider AWS 域名 Provider。
type Provider struct {
	cred   provider.Credential
	region string
}

// ListDomains 拉取注册域名与托管 Zone。
func (p *Provider) ListDomains(ctx context.Context) ([]provider.DomainInfo, error) {
	var out []provider.DomainInfo
	var errs []string

	sdkCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion(p.region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			p.cred.AccessKey, p.cred.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}

	// 1. 注册域名（Route 53 Domains）
	domainsClient := route53domains.NewFromConfig(sdkCfg)
	paginator := route53domains.NewListDomainsPaginator(domainsClient, &route53domains.ListDomainsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			errs = append(errs, fmt.Sprintf("route53domains: %v", err))
			break
		}
		for _, d := range page.Domains {
			info := provider.DomainInfo{
				Name:      *d.DomainName,
				Kind:      provider.KindDomain,
				Registrar: "AWS Route 53",
				Status:    "ok",
			}
			if d.Expiry != nil {
				info.ExpireAt = *d.Expiry
			}
			out = append(out, info)
		}
	}

	// 2. 托管 Zone（Route 53）
	zonesClient := route53.NewFromConfig(sdkCfg)
	zPaginator := route53.NewListHostedZonesPaginator(zonesClient, &route53.ListHostedZonesInput{})
	for zPaginator.HasMorePages() {
		page, err := zPaginator.NextPage(ctx)
		if err != nil {
			errs = append(errs, fmt.Sprintf("route53: %v", err))
			break
		}
		for _, z := range page.HostedZones {
			out = append(out, provider.DomainInfo{
				Name:      *z.Name,
				Kind:      provider.KindZone,
				Registrar: "AWS Route 53",
				Status:    *z.CallerReference,
			})
		}
	}

	if out == nil && len(errs) > 0 {
		return nil, fmt.Errorf("AWS API 调用失败: %s", fmt.Sprint(errs))
	}
	return out, nil
}

// CheckConnection 连通性检测：Route 53 托管 Zone 列表拉取。
func (p *Provider) CheckConnection(ctx context.Context) error {
	maxItems := int32(1)
	sdkCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion(p.region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			p.cred.AccessKey, p.cred.SecretKey, "")),
	)
	if err != nil {
		return fmt.Errorf("加载 AWS 配置失败: %w", err)
	}
	client := route53.NewFromConfig(sdkCfg)
	_, err = client.ListHostedZones(ctx, &route53.ListHostedZonesInput{MaxItems: &maxItems})
	if err != nil {
		return fmt.Errorf("AWS 连接失败: %w", err)
	}
	return nil
}
