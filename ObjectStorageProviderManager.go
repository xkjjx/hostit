package main

type ObjectStorageProviderManager interface {
	InstantiateClient() error
	VerifyNamespace() (bool, error)
	CreateStorageInstance() error
	UploadFiles() error
}
