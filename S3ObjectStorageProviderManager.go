package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmTypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfrontTypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	route53Types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type S3ObjectStorageProviderManager struct {
	domainName                       string
	folderName                       string
	awsAccountNumber                 string
	s3Client                         *s3.Client
	cloudfrontClient                 *cloudfront.Client
	acmClientUsEast1                 *acm.Client
	cloudfrontDistributionDomainName string
	cloudfrontDistributionId         string
	certificateArn                   string
	acmValidationRecords             []*route53Types.ResourceRecordSet
}

// HTTPS finalization is disabled for now; handled externally

func (s3ObjectStorageProviderManager *S3ObjectStorageProviderManager) InstantiateClient() error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("issue with getting credentials")
	}
	stsClient := sts.NewFromConfig(cfg)
	result, err := stsClient.GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return errors.New("issue with user information")
	}
	fmt.Printf("Using AWS Account %s for object storage provider\n", *result.Account)
	s3ObjectStorageProviderManager.awsAccountNumber = *result.Account
	s3ObjectStorageProviderManager.s3Client = s3.NewFromConfig(cfg)
	s3ObjectStorageProviderManager.cloudfrontClient = cloudfront.NewFromConfig(cfg)
	// CloudFront requires ACM certificates in us-east-1
	acmCfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-east-1"))
	if err != nil {
		return fmt.Errorf("issue configuring ACM client: %w", err)
	}
	s3ObjectStorageProviderManager.acmClientUsEast1 = acm.NewFromConfig(acmCfg)
	return nil
}

func (s3ObjectStorageProviderManager S3ObjectStorageProviderManager) VerifyNamespace() (bool, error) {
	return true, nil
}

func (s3ObjectStorageProviderManager S3ObjectStorageProviderManager) CreateStorageInstance() error {
	if s3ObjectStorageProviderManager.s3Client == nil {
		return errors.New("s3 client not instantiated")
	}
	bucketName := fmt.Sprintf("%s-%s-hostit", s3ObjectStorageProviderManager.awsAccountNumber, s3ObjectStorageProviderManager.domainName)
	_, err := s3ObjectStorageProviderManager.s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: &bucketName,
	})
	if err != nil {
		return errors.New("issue with creating bucket")
	}

	securityPolicyBoolean := true
	_, err = s3ObjectStorageProviderManager.s3Client.PutPublicAccessBlock(context.TODO(), &s3.PutPublicAccessBlockInput{
		Bucket: &bucketName,
		PublicAccessBlockConfiguration: &s3Types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       &securityPolicyBoolean,
			IgnorePublicAcls:      &securityPolicyBoolean,
			BlockPublicPolicy:     &securityPolicyBoolean,
			RestrictPublicBuckets: &securityPolicyBoolean,
		},
	})

	if err != nil {
		return errors.New("issue with setting security policy in s3 bucket")
	}
	return nil
}

func (s3ObjectStorageProviderManager S3ObjectStorageProviderManager) UploadFilesToNewInstance() error {
	const maxFileSizeBytes int64 = 1 * 1024 * 1024 * 1024 // 1GB

	if s3ObjectStorageProviderManager.s3Client == nil {
		return errors.New("s3 client not instantiated")
	}
	if s3ObjectStorageProviderManager.awsAccountNumber == "" {
		return errors.New("aws account number not set; call InstantiateClient first")
	}

	bucketName := fmt.Sprintf("%s-%s-hostit", s3ObjectStorageProviderManager.awsAccountNumber, s3ObjectStorageProviderManager.domainName)

	finder := NewUploadFileFinder()
	filesToUpload, err := finder.FindFiles(s3ObjectStorageProviderManager.folderName, maxFileSizeBytes)
	if err != nil {
		return err
	}

	ctx := context.Background()
	for _, repoPath := range filesToUpload {
		fullPath := filepath.Join(s3ObjectStorageProviderManager.folderName, filepath.FromSlash(repoPath))

		f, err := os.Open(fullPath)
		if err != nil {
			return fmt.Errorf("failed to open '%s': %w", fullPath, err)
		}
		_, putErr := s3ObjectStorageProviderManager.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &bucketName,
			Key:    &repoPath,
			Body:   f,
		})
		closeErr := f.Close()
		if putErr != nil {
			return fmt.Errorf("failed to upload '%s': %w", repoPath, putErr)
		}
		if closeErr != nil {
			return fmt.Errorf("failed to close file '%s': %w", fullPath, closeErr)
		}
	}
	return nil
}

func (s3ObjectStorageProviderManager *S3ObjectStorageProviderManager) CreateAvailableDomain() error {
	if s3ObjectStorageProviderManager.cloudfrontClient == nil || s3ObjectStorageProviderManager.s3Client == nil {
		return errors.New("aws clients not instantiated")
	}

	bucketName := fmt.Sprintf("%s-%s-hostit", s3ObjectStorageProviderManager.awsAccountNumber, s3ObjectStorageProviderManager.domainName)
	ctx := context.Background()

	// Create Origin Access Control for the S3 origin
	oacName := fmt.Sprintf("hostit-oac-%s", bucketName)
	oacOut, err := s3ObjectStorageProviderManager.cloudfrontClient.CreateOriginAccessControl(ctx, &cloudfront.CreateOriginAccessControlInput{
		OriginAccessControlConfig: &cloudfrontTypes.OriginAccessControlConfig{
			Name:                          aws.String(oacName),
			Description:                   aws.String("Hostit OAC for S3 origin"),
			OriginAccessControlOriginType: cloudfrontTypes.OriginAccessControlOriginTypesS3,
			SigningBehavior:               cloudfrontTypes.OriginAccessControlSigningBehaviorsAlways,
			SigningProtocol:               cloudfrontTypes.OriginAccessControlSigningProtocolsSigv4,
		},
	})
	if err != nil {
		return fmt.Errorf("failed creating Origin Access Control: %w", err)
	}

	originId := "s3-origin"
	s3Domain := fmt.Sprintf("%s.s3.amazonaws.com", bucketName)
	createDistOut, err := s3ObjectStorageProviderManager.cloudfrontClient.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: &cloudfrontTypes.DistributionConfig{
			CallerReference:   aws.String(fmt.Sprintf("hostit-%s", bucketName)),
			Comment:           aws.String("Hostit distribution for S3 static site"),
			Enabled:           aws.Bool(true),
			DefaultRootObject: aws.String("index.html"),
			Origins: &cloudfrontTypes.Origins{
				Quantity: aws.Int32(1),
				Items: []cloudfrontTypes.Origin{
					{
						Id:         aws.String(originId),
						DomainName: aws.String(s3Domain),
						S3OriginConfig: &cloudfrontTypes.S3OriginConfig{
							OriginAccessIdentity: aws.String(""),
						},
						OriginAccessControlId: oacOut.OriginAccessControl.Id,
					},
				},
			},
			DefaultCacheBehavior: &cloudfrontTypes.DefaultCacheBehavior{
				TargetOriginId:       aws.String(originId),
				ViewerProtocolPolicy: cloudfrontTypes.ViewerProtocolPolicyRedirectToHttps,
				Compress:             aws.Bool(true),
				AllowedMethods: &cloudfrontTypes.AllowedMethods{
					Quantity: aws.Int32(3),
					Items:    []cloudfrontTypes.Method{cloudfrontTypes.MethodGet, cloudfrontTypes.MethodHead, cloudfrontTypes.MethodOptions},
					CachedMethods: &cloudfrontTypes.CachedMethods{
						Quantity: aws.Int32(2),
						Items:    []cloudfrontTypes.Method{cloudfrontTypes.MethodGet, cloudfrontTypes.MethodHead},
					},
				},
				ForwardedValues: &cloudfrontTypes.ForwardedValues{
					QueryString: aws.Bool(false),
					Cookies: &cloudfrontTypes.CookiePreference{
						Forward: cloudfrontTypes.ItemSelectionNone,
					},
				},
				MinTTL: aws.Int64(0),
			},
			ViewerCertificate: &cloudfrontTypes.ViewerCertificate{
				CloudFrontDefaultCertificate: aws.Bool(true),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed creating CloudFront distribution: %w", err)
	}
	if createDistOut == nil || createDistOut.Distribution == nil || createDistOut.Distribution.DomainName == nil || createDistOut.Distribution.Id == nil {
		return errors.New("unexpected empty distribution response")
	}

	// Persist distribution details for DNS generation
	s3ObjectStorageProviderManager.cloudfrontDistributionDomainName = *createDistOut.Distribution.DomainName
	s3ObjectStorageProviderManager.cloudfrontDistributionId = *createDistOut.Distribution.Id

	// Attach S3 bucket policy to allow CloudFront (OAC) to read objects
	bucketArn := fmt.Sprintf("arn:aws:s3:::%s", bucketName)
	objectsArn := fmt.Sprintf("%s/*", bucketArn)
	distributionArn := fmt.Sprintf("arn:aws:cloudfront::%s:distribution/%s", s3ObjectStorageProviderManager.awsAccountNumber, s3ObjectStorageProviderManager.cloudfrontDistributionId)
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Sid": "AllowCloudFrontRead",
				"Effect": "Allow",
				"Principal": {"Service": "cloudfront.amazonaws.com"},
				"Action": ["s3:GetObject"],
				"Resource": ["%s"],
				"Condition": {
					"StringEquals": {"AWS:SourceArn": "%s"}
				}
			}
		]
	}`, objectsArn, distributionArn)

	_, err = s3ObjectStorageProviderManager.s3Client.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: aws.String(bucketName),
		Policy: aws.String(policy),
	})
	if err != nil {
		return fmt.Errorf("failed attaching S3 bucket policy for OAC: %w", err)
	}

	// 4) Request ACM certificate for the custom domain (DNS validation)
	if s3ObjectStorageProviderManager.acmClientUsEast1 == nil {
		return errors.New("acm client not instantiated")
	}
	certOut, err := s3ObjectStorageProviderManager.acmClientUsEast1.RequestCertificate(ctx, &acm.RequestCertificateInput{
		DomainName:       aws.String(s3ObjectStorageProviderManager.domainName),
		ValidationMethod: acmTypes.ValidationMethodDns,
	})
	if err != nil {
		return fmt.Errorf("failed to request ACM certificate: %w", err)
	}
	if certOut == nil || certOut.CertificateArn == nil || *certOut.CertificateArn == "" {
		return errors.New("unexpected empty certificate arn from ACM")
	}
	s3ObjectStorageProviderManager.certificateArn = *certOut.CertificateArn

	// Fetch DNS validation records for the certificate
	descOut, err := s3ObjectStorageProviderManager.acmClientUsEast1.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: certOut.CertificateArn,
	})
	if err != nil {
		return fmt.Errorf("failed to describe ACM certificate: %w", err)
	}
	var validationRRSets []*route53Types.ResourceRecordSet
	if descOut != nil && descOut.Certificate != nil {
		for _, dvo := range descOut.Certificate.DomainValidationOptions {
			if dvo.ResourceRecord == nil || dvo.ResourceRecord.Name == nil || dvo.ResourceRecord.Value == nil {
				continue
			}
			rrSet := &route53Types.ResourceRecordSet{
				Name: dvo.ResourceRecord.Name,
				Type: route53Types.RRTypeCname,
				TTL:  aws.Int64(300),
				ResourceRecords: []route53Types.ResourceRecord{
					{Value: dvo.ResourceRecord.Value},
				},
			}
			validationRRSets = append(validationRRSets, rrSet)
		}
	}
	s3ObjectStorageProviderManager.acmValidationRecords = validationRRSets

	return nil
}

func (s3ObjectStorageProviderManager *S3ObjectStorageProviderManager) FinalizeHttps() error {
	return nil
}

func (s3ObjectStorageProviderManager S3ObjectStorageProviderManager) GetRequiredDnsRecords() ([]*route53Types.ResourceRecordSet, error) {
	if s3ObjectStorageProviderManager.cloudfrontDistributionDomainName == "" {
		return nil, errors.New("cloudfront distribution not created")
	}
	records := []*route53Types.ResourceRecordSet{
		{
			Name: aws.String(s3ObjectStorageProviderManager.domainName + "."),
			Type: route53Types.RRTypeCname,
			TTL:  aws.Int64(300),
			ResourceRecords: []route53Types.ResourceRecord{
				{Value: aws.String(s3ObjectStorageProviderManager.cloudfrontDistributionDomainName)},
			},
		},
	}
	// Append ACM DNS validation records if any
	if len(s3ObjectStorageProviderManager.acmValidationRecords) > 0 {
		records = append(records, s3ObjectStorageProviderManager.acmValidationRecords...)
	}
	return records, nil
}

func NewS3ObjectStorageProviderManager(domainName string, folderName string) (*S3ObjectStorageProviderManager, error) {
	return &S3ObjectStorageProviderManager{
		domainName:                       domainName,
		folderName:                       folderName,
		awsAccountNumber:                 "",
		s3Client:                         nil,
		cloudfrontClient:                 nil,
		acmClientUsEast1:                 nil,
		cloudfrontDistributionDomainName: "",
		cloudfrontDistributionId:         "",
		certificateArn:                   "",
		acmValidationRecords:             nil,
	}, nil
}
