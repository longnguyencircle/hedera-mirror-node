/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019 - 2021 Hedera Hashgraph, LLC
 * ​
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * ‍
 */

package construction

import (
	"testing"

	rTypes "github.com/coinbase/rosetta-sdk-go/types"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/config"
	"github.com/hashgraph/hedera-mirror-node/hedera-mirror-rosetta/test/mocks/repository"
	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

func TestTokenFreezeUnfreezeTransactionConstructorSuite(t *testing.T) {
	suite.Run(t, new(tokenFreezeUnfreezeTransactionConstructorSuite))
}

type tokenFreezeUnfreezeTransactionConstructorSuite struct {
	suite.Suite
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestNewTokenFreezeTransactionConstructor() {
	h := newTokenFreezeTransactionConstructor(&repository.MockTokenRepository{})
	assert.NotNil(suite.T(), h)
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestNewTokenUnfreezeTransactionConstructor() {
	h := newTokenUnfreezeTransactionConstructor(&repository.MockTokenRepository{})
	assert.NotNil(suite.T(), h)
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestGetOperationType() {
	var tests = []struct {
		name       string
		newHandler newConstructorFunc
		expected   string
	}{
		{
			name:       "TokenFreezeTransactionConstructor",
			newHandler: newTokenFreezeTransactionConstructor,
			expected:   config.OperationTypeTokenFreeze,
		},
		{
			name:       "TokenUnfreezeTransactionConstructor",
			newHandler: newTokenUnfreezeTransactionConstructor,
			expected:   config.OperationTypeTokenUnfreeze,
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			h := tt.newHandler(&repository.MockTokenRepository{})
			assert.Equal(t, tt.expected, h.GetOperationType())
		})
	}
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestGetSdkTransactionType() {
	var tests = []struct {
		name       string
		newHandler newConstructorFunc
		expected   string
	}{
		{
			name:       "TokenFreezeTransactionConstructor",
			newHandler: newTokenFreezeTransactionConstructor,
			expected:   "TokenFreezeTransaction",
		},
		{
			name:       "TokenUnfreezeTransactionConstructor",
			newHandler: newTokenUnfreezeTransactionConstructor,
			expected:   "TokenUnfreezeTransaction",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			h := tt.newHandler(&repository.MockTokenRepository{})
			assert.Equal(t, tt.expected, h.GetSdkTransactionType())
		})
	}
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestConstruct() {
	var tests = []struct {
		name             string
		updateOperations updateOperationsFunc
		expectError      bool
	}{
		{
			name: "Success",
		},
		{
			name: "EmptyOperations",
			updateOperations: func([]*rTypes.Operation) []*rTypes.Operation {
				return make([]*rTypes.Operation, 0)
			},
			expectError: true,
		},
	}

	runTests := func(t *testing.T, operationType string, newHandler newConstructorFunc) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				operations := getFreezeUnfreezeOperations(operationType)
				mockTokenRepo := &repository.MockTokenRepository{}
				h := newHandler(mockTokenRepo)
				configMockTokenRepo(mockTokenRepo, defaultMockTokenRepoConfigs[0])

				if tt.updateOperations != nil {
					operations = tt.updateOperations(operations)
				}

				// when
				tx, signers, err := h.Construct(nodeAccountId, operations)

				// then
				if tt.expectError {
					assert.NotNil(t, err)
					assert.Nil(t, signers)
					assert.Nil(t, tx)
				} else {
					assert.Nil(t, err)
					assert.ElementsMatch(t, []hedera.AccountID{payerId}, signers)
					assertTokenFreezeUnfreezeTransaction(t, operations[0], nodeAccountId, tx)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenFreezeTransactionConstructor", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenFreeze, newTokenFreezeTransactionConstructor)
	})

	suite.T().Run("TokenUnfreezeTransactionConstructor", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenUnfreeze, newTokenUnfreezeTransactionConstructor)
	})
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestParse() {
	tokenFreezeTransaction := hedera.NewTokenFreezeTransaction().
		SetAccountID(accountId).
		SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
		SetTokenID(tokenIdA).
		SetTransactionID(hedera.TransactionIDGenerate(payerId))
	tokenUnfreezeTransaction := hedera.NewTokenUnfreezeTransaction().
		SetAccountID(accountId).
		SetNodeAccountIDs([]hedera.AccountID{nodeAccountId}).
		SetTokenID(tokenIdA).
		SetTransactionID(hedera.TransactionIDGenerate(payerId))

	defaultGetTransaction := func(operationType string) ITransaction {
		if operationType == config.OperationTypeTokenFreeze {
			return tokenFreezeTransaction
		}

		return tokenUnfreezeTransaction
	}

	var tests = []struct {
		name           string
		tokenRepoErr   bool
		getTransaction func(operationType string) ITransaction
		expectError    bool
	}{
		{
			name:           "Success",
			getTransaction: defaultGetTransaction,
		},
		{
			name:           "TokenNotFound",
			tokenRepoErr:   true,
			getTransaction: defaultGetTransaction,
			expectError:    true,
		},
		{
			name: "InvalidTransaction",
			getTransaction: func(operationType string) ITransaction {
				return hedera.NewTransferTransaction()
			},
			expectError: true,
		},
		{
			name: "TransactionMismatch",
			getTransaction: func(operationType string) ITransaction {
				if operationType == config.OperationTypeTokenFreeze {
					return tokenUnfreezeTransaction
				}

				return tokenFreezeTransaction
			},
			expectError: true,
		},
	}

	runTests := func(t *testing.T, operationType string, newHandler newConstructorFunc) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				expectedOperations := getFreezeUnfreezeOperations(operationType)

				mockTokenRepo := &repository.MockTokenRepository{}
				h := newHandler(mockTokenRepo)
				tx := tt.getTransaction(operationType)

				if tt.tokenRepoErr {
					configMockTokenRepo(mockTokenRepo, mockTokenRepoNotFoundConfigs[0])
				} else {
					configMockTokenRepo(mockTokenRepo, defaultMockTokenRepoConfigs[0])
				}

				// when
				operations, signers, err := h.Parse(tx)

				// then
				if tt.expectError {
					assert.NotNil(t, err)
					assert.Nil(t, operations)
					assert.Nil(t, signers)
				} else {
					assert.Nil(t, err)
					assert.ElementsMatch(t, []hedera.AccountID{payerId}, signers)
					assert.ElementsMatch(t, expectedOperations, operations)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenFreezeTransactionConstructor", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenFreeze, newTokenFreezeTransactionConstructor)
	})

	suite.T().Run("TokenUnfreezeTransactionConstructor", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenUnfreeze, newTokenUnfreezeTransactionConstructor)
	})
}

func (suite *tokenFreezeUnfreezeTransactionConstructorSuite) TestPreprocess() {
	var tests = []struct {
		name             string
		tokenRepoErr     bool
		updateOperations updateOperationsFunc
		expectError      bool
	}{
		{
			name:             "Success",
			updateOperations: nil,
			expectError:      false,
		},
		{
			name: "NoOperationMetadata",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Metadata = nil
				return operations
			},
			expectError: true,
		},
		{
			name: "ZeroAccountId",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Metadata["account"] = "0.0.0"
				return operations
			},
			expectError: true,
		},
		{
			name: "InvalidOperationMetadata",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Metadata = map[string]interface{}{
					"account": "x.y.z",
				}
				return operations
			},
			expectError: true,
		},
		{
			name: "InvalidAccountAddress",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Account.Address = "x.y.z"
				return operations
			},
			expectError: true,
		},
		{
			name: "TokenDecimalsMismatch",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Amount.Currency.Decimals = 1990
				return operations
			},
			expectError: true,
		},
		{
			name:         "TokenNotFound",
			tokenRepoErr: true,
			expectError:  true,
		},
		{
			name: "MultipleOperations",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				return append(operations, &rTypes.Operation{})
			},
			expectError: true,
		},
		{
			name: "InvalidOperationType",
			updateOperations: func(operations []*rTypes.Operation) []*rTypes.Operation {
				operations[0].Type = config.OperationTypeCryptoTransfer
				return operations
			},
			expectError: true,
		},
	}

	runTests := func(t *testing.T, operationType string, newHandler newConstructorFunc) {
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				operations := getFreezeUnfreezeOperations(operationType)

				mockTokenRepo := &repository.MockTokenRepository{}
				h := newHandler(mockTokenRepo)

				if tt.tokenRepoErr {
					configMockTokenRepo(mockTokenRepo, mockTokenRepoNotFoundConfigs[0])
				} else {
					configMockTokenRepo(mockTokenRepo, defaultMockTokenRepoConfigs[0])
				}

				if tt.updateOperations != nil {
					operations = tt.updateOperations(operations)
				}

				// when
				signers, err := h.Preprocess(operations)

				// then
				if tt.expectError {
					assert.NotNil(t, err)
					assert.Nil(t, signers)
				} else {
					assert.Nil(t, err)
					assert.ElementsMatch(t, []hedera.AccountID{payerId}, signers)
					mockTokenRepo.AssertExpectations(t)
				}
			})
		}
	}

	suite.T().Run("TokenFreezeTransactionConstructor", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenFreeze, newTokenFreezeTransactionConstructor)
	})

	suite.T().Run("TokenUnfreezeTransactionConstructor", func(t *testing.T) {
		runTests(t, config.OperationTypeTokenUnfreeze, newTokenUnfreezeTransactionConstructor)
	})
}

func assertTokenFreezeUnfreezeTransaction(
	t *testing.T,
	operation *rTypes.Operation,
	nodeAccountId hedera.AccountID,
	actual ITransaction,
) {
	if operation.Type == config.OperationTypeTokenFreeze {
		assert.IsType(t, &hedera.TokenFreezeTransaction{}, actual)
	} else {
		assert.IsType(t, &hedera.TokenUnfreezeTransaction{}, actual)
	}

	var account string
	var payer string
	var token string

	switch tx := actual.(type) {
	case *hedera.TokenFreezeTransaction:
		account = tx.GetAccountID().String()
		payer = tx.GetTransactionID().AccountID.String()
		token = tx.GetTokenID().String()
	case *hedera.TokenUnfreezeTransaction:
		account = tx.GetAccountID().String()
		payer = tx.GetTransactionID().AccountID.String()
		token = tx.GetTokenID().String()
	}

	assert.Equal(t, operation.Metadata["account"], account)
	assert.Equal(t, operation.Account.Address, payer)
	assert.Equal(t, operation.Amount.Currency.Symbol, token)
	assert.ElementsMatch(t, []hedera.AccountID{nodeAccountId}, actual.GetNodeAccountIDs())
}

func getFreezeUnfreezeOperations(operationType string) []*rTypes.Operation {
	return []*rTypes.Operation{
		{
			OperationIdentifier: &rTypes.OperationIdentifier{Index: 0},
			Type:                operationType,
			Account:             &rTypes.AccountIdentifier{Address: payerId.String()},
			Amount: &rTypes.Amount{
				Value:    "0",
				Currency: dbTokenA.ToRosettaCurrency(),
			},
			Metadata: map[string]interface{}{
				"account": accountId.String(),
			},
		},
	}
}
