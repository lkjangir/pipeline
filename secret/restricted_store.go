package secret

import (
	"fmt"

	secretTypes "github.com/banzaicloud/pipeline/pkg/secret"
)

// restrictedSecretStore checks whether the user can access a certain secret.
// For now this only means checking for forbidden tags.
type restrictedSecretStore struct {
	*secretStore
}

func (s *restrictedSecretStore) List(orgid uint, query *secretTypes.ListSecretsQuery) ([]*SecretItemResponse, error) {
	responseItems, err := s.secretStore.List(orgid, query)
	if err != nil {
		return nil, err
	}

	var newResponseItems []*SecretItemResponse

	for _, item := range responseItems {
		if HasForbiddenTag(item.Tags) == nil {
			newResponseItems = append(newResponseItems, item)
		}
	}

	return newResponseItems, nil
}

func (s *restrictedSecretStore) Update(organizationID uint, secretID string, value *CreateSecretRequest) error {
	if err := s.checkBlockingTags(organizationID, secretID); err != nil {
		return err
	}

	return s.secretStore.Update(organizationID, secretID, value)
}

func (s *restrictedSecretStore) Delete(organizationID uint, secretID string) error {
	if err := s.checkBlockingTags(organizationID, secretID); err != nil {
		return err
	}

	return s.secretStore.Delete(organizationID, secretID)
}

func (s *restrictedSecretStore) checkBlockingTags(organizationID uint, secretID string) error {

	secretItem, err := s.secretStore.Get(organizationID, secretID)
	if err != nil {
		return err
	}

	// check forbidden tags
	if err := HasForbiddenTag(secretItem.Tags); err != nil {
		return err
	}

	// check read only tag
	if err := s.isSecretReadOnly(secretItem); err != nil {
		return err
	}

	return nil
}

func (s *restrictedSecretStore) checkForbiddenTags(organizationID uint, secretID string) error {
	secretItem, err := s.secretStore.Get(organizationID, secretID)
	if err != nil {
		return err
	}

	return HasForbiddenTag(secretItem.Tags)
}

func (s *restrictedSecretStore) isSecretReadOnly(secretItem *SecretItemResponse) error {
	for _, tag := range secretItem.Tags {
		if tag == secretTypes.TagBanzaiReadonly {
			return ReadOnlyError{
				SecretID: secretItem.ID,
			}
		}
	}

	return nil

}

// ReadOnlyError describes a secret error where it contains read only tag
type ReadOnlyError struct {
	SecretID string
}

func (roe ReadOnlyError) Error() string {
	return fmt.Sprintf("secret [%s] is read only, cannot be updated/deleted", roe.SecretID)
}

// ForbiddenError describes a secret error where it contains forbidden tag
type ForbiddenError struct {
	ForbiddenTag string
}

func (f ForbiddenError) Error() string {
	return fmt.Sprintf("secret contains a forbidden tag: %s", f.ForbiddenTag)
}

// HasForbiddenTag is looking for forbidden tags
func HasForbiddenTag(tags []string) error {
	for _, tag := range tags {
		for _, forbiddenTag := range secretTypes.ForbiddenTags {
			if tag == forbiddenTag {
				return ForbiddenError{
					ForbiddenTag: tag,
				}
			}
		}
	}
	return nil
}
