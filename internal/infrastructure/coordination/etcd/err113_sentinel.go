package etcd

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrEtcdAtLeastOneEndpointIsRequired                      = errors.New("etcd: at least one endpoint is required")
	ErrEtcdEndpointMustNotBeBlank                            = errors.New("etcd: endpoint must not be blank")
	ErrEtcdDialTimeoutMustBePositive                         = errors.New("etcd: dial timeout must be positive")
	ErrEtcdElectionTTLMustBePositive                         = errors.New("etcd: election TTL must be positive")
	ErrEtcdLeaderKeyPrefixMustNotBeBlank                     = errors.New("etcd: leader key prefix must not be blank")
	ErrEtcdElectionRequiresAnInitializedClient               = errors.New("etcd: election requires an initialized client")
	ErrEtcdElectionRequiresAKeyPrefix                        = errors.New("etcd: election requires a key prefix")
	ErrEtcdElectionRequiresAPositiveTTL                      = errors.New("etcd: election requires a positive TTL")
	ErrEtcdElectionIsNotInitialized                          = errors.New("etcd: election is not initialized")
	ErrEtcdCandidateIDMustNotBeBlank                         = errors.New("etcd: candidate id must not be blank")
	ErrEtcdNoActiveLeadershipToResign                        = errors.New("etcd: no active leadership to resign")
	ErrEtcdWatcherRequiresAnInitializedClient                = errors.New("etcd: watcher requires an initialized client")
	ErrEtcdWatcherIsNotInitialized                           = errors.New("etcd: watcher is not initialized")
	ErrEtcdWatchPrefixMustNotBeBlank                         = errors.New("etcd: watch prefix must not be blank")
	ErrEtcdWatchHandlerIsRequired                            = errors.New("etcd: watch handler is required")
	ErrEtcdClientIsNotInitialized                            = errors.New("etcd: client is not initialized")
	ErrEtcdKeyNotFound                                       = errors.New("etcd: key not found")
	ErrEtcdAlreadyCampaigningForResignBeforeCampaigningAgain = errors.New("etcd: already campaigning for; resign before campaigning again")
	ErrEtcdNoLeaderElectedFor                                = errors.New("etcd: no leader elected for")
	ErrEtcdWatchStreamClosedFor                              = errors.New("etcd: watch stream closed for")
)
