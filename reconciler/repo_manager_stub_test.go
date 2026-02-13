package reconciler

import "github.com/crmarques/declarest/resource"

type stubRepoManager struct {
	initErr    error
	initCalled bool
	paths      []string
	applyCalls []string
}

func (s *stubRepoManager) Init() error {
	s.initCalled = true
	return s.initErr
}

func (s *stubRepoManager) GetResource(string) (resource.Resource, error) {
	return resource.Resource{}, nil
}

func (s *stubRepoManager) CreateResource(string, resource.Resource) error {
	return nil
}

func (s *stubRepoManager) UpdateResource(string, resource.Resource) error {
	return nil
}

func (s *stubRepoManager) ApplyResource(path string, _ resource.Resource) error {
	s.applyCalls = append(s.applyCalls, path)
	return nil
}

func (s *stubRepoManager) DeleteResource(string) error {
	return nil
}

func (s *stubRepoManager) GetResourceCollection(string) ([]resource.Resource, error) {
	return nil, nil
}

func (s *stubRepoManager) ListResourcePaths() []string {
	return s.paths
}

func (s *stubRepoManager) Close() error {
	return nil
}

type remoteCheckerRepo struct {
	stubRepoManager
	configured bool
	err        error
}

func (r *remoteCheckerRepo) CheckRemoteAccess() (bool, error) {
	return r.configured, r.err
}

type localInitCheckerRepo struct {
	stubRepoManager
	initialized bool
	err         error
}

func (l *localInitCheckerRepo) IsLocalRepositoryInitialized() (bool, error) {
	return l.initialized, l.err
}

type remoteSyncCheckerRepo struct {
	stubRepoManager
	supported bool
	inSync    bool
	err       error
}

func (r *remoteSyncCheckerRepo) CheckRemoteSync() (bool, bool, error) {
	return r.supported, r.inSync, r.err
}

type syncRepoManager struct {
	stubRepoManager
	syncErr    error
	syncCalled bool
}

func (s *syncRepoManager) SyncLocalFromRemoteIfConfigured() error {
	s.syncCalled = true
	return s.syncErr
}
