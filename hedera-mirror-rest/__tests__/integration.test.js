/*-
 * ‌
 * Hedera Mirror Node
 * ​
 * Copyright (C) 2019-2020 Hedera Hashgraph, LLC
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
'use strict';

/**
 * Integration tests for the rest-api and postgresql database.
 * Tests will be performed using either a docker postgres instance managed by the testContainers module or
 * some other database (running locally, instantiated in the CI environment, etc).
 * Tests instantiate the database schema via a flywaydb wrapper using the flyway CLI to clean and migrate the
 * schema (using sql files in the ../src/resources/db/migration directory).
 *
 * Test data is created by:
 * 1) reading account id, balance, expiration and crypto transfer information from integration_test_data.json
 * 2) storing those accounts in integration DB
 * 3) creating 3 balances records per account at timestamp 1000, 2000, 3000 in the integration DB
 * 4) apply transfers (from integration_test_data.json) to the integration DB
 *
 * Tests are then run in code below (find TESTS all caps) and by comparing requests/responses from the server to data
 * in the specs/ dir.
 *
 * environment variables used:
 * TEST_DB_HOST (default: use testcontainers, examples: localhost, dbhost, 10.0.0.75)
 * TEST_DB_PORT (default: 5432)
 * TEST_DB_NAME (default: mirror_node_integration)
 */
const transactions = require('../transactions.js');
const path = require('path');
const request = require('supertest');
const server = require('../server');
const fs = require('fs');
const integrationDbOps = require('./integrationDbOps.js');

beforeAll(async () => {
  jest.setTimeout(20000);
  await integrationDbOps.instantiateDatabase();
});

afterAll(() => {
  integrationDbOps.closeConnection();
});

let accountEntityIds = {};
beforeEach(async () => {
  await integrationDbOps.cleanUp();
  accountEntityIds = {};
  await setupData();
});

//
// TEST DATA
// shard 0, realm 15, accounts 1-10
// 3 balances per account
// several transactions
//
const shard = 0;
const realm = 15;
const addAccount = async function(accountId, exp_tm_nanosecs = null) {
  let e = accountEntityIds[accountId];
  if (e) {
    return e;
  }
  let res = await integrationDbOps.runSqlQuery(
    'insert into t_entities (fk_entity_type_id, entity_shard, entity_realm, entity_num, exp_time_ns) values ($1, $2, $3, $4, $5) returning id;',
    [1, shard, realm, accountId, exp_tm_nanosecs]
  );
  e = res.rows[0]['id'];
  accountEntityIds[accountId] = e;
  return e;
};

const addTransaction = async function(
  consensusTimestamp,
  fileId,
  payerAccountId,
  transfers,
  validDurationSeconds = 11,
  maxFee = 33,
  result = 22,
  type = 14
) {
  await integrationDbOps.runSqlQuery(
    'insert into t_transactions (consensus_ns, valid_start_ns, fk_rec_file_id, fk_payer_acc_id, fk_node_acc_id, result, type, valid_duration_seconds, max_fee) values ($1, $2, $3, $4, $5, $6, $7, $8, $9);',
    [
      consensusTimestamp,
      consensusTimestamp - 1,
      fileId,
      accountEntityIds[payerAccountId],
      accountEntityIds[2],
      result,
      type,
      validDurationSeconds,
      maxFee
    ]
  );
  for (var i = 0; i < transfers.length; ++i) {
    let xfer = transfers[i];
    await integrationDbOps.runSqlQuery(
      'insert into t_cryptotransferlists (consensus_timestamp, amount, realm_num, entity_num) values ($1, $2, $3, $4);',
      [consensusTimestamp, xfer[1], realm, xfer[0]]
    );
  }
};

/**
 * Add a crypto transfer from A to B with A also paying 1 tinybar to account number 2 (fee)
 * @param consensusTimestamp
 * @param fileId
 * @param payerAccountId
 * @param recipientAccountId
 * @param amount
 * @returns {Promise<void>}
 */
const addCryptoTransferTransaction = async function(
  consensusTimestamp,
  fileId,
  payerAccountId,
  recipientAccountId,
  amount,
  validDurationSeconds,
  maxFee,
  bankId = 2
) {
  await addTransaction(
    consensusTimestamp,
    fileId,
    payerAccountId,
    [
      [payerAccountId, -1 - amount],
      [recipientAccountId, amount],
      [bankId, 1]
    ],
    validDurationSeconds,
    maxFee
  );
};

/**
 * Setup test data in the postgres instance.
 */
const testDataPath = path.join(__dirname, 'integration_test_data.json');
const testData = fs.readFileSync(testDataPath);
const testDataJson = JSON.parse(testData);

const setupData = async function() {
  let res = await integrationDbOps.runSqlQuery('insert into t_record_files (name) values ($1) returning id;', ['test']);
  let fileId = res.rows[0]['id'];
  console.log(`Record file id is ${fileId}`);

  const balancePerAccountCount = 3;
  const testAccounts = testDataJson['accounts'];
  console.log(`Adding ${testAccounts.length} accounts with ${balancePerAccountCount} balances per account`);
  for (let account of testAccounts) {
    await addAccount(account.id, account.expiration_ns);

    // Add 3 balances for each account.
    for (var ts = 0; ts < balancePerAccountCount; ++ts) {
      await integrationDbOps.runSqlQuery(
        'insert into account_balances (consensus_timestamp, account_realm_num, account_num, balance) values ($1, $2, $3, $4);',
        [ts * 1000, realm, account.id, account.balance]
      );
    }
  }

  console.log('Adding crypto transfer transactions');
  for (let transfer of testDataJson['transfers']) {
    await addCryptoTransferTransaction(transfer.time, fileId, transfer.from, transfer.to, transfer.amount);
  }

  console.log('Finished initializing DB data');
};

/**
 * Map a DB transaction/cryptotransfer result to something easily comparable in a test assert/expect.
 * @param rows
 * @returns {*}
 */
function mapTransactionResults(rows) {
  return rows.map(function(v) {
    return '@' + v['consensus_ns'] + ': account ' + v['account_num'] + ' \u0127' + v['amount'];
  });
}

function extractDurationAndMaxFeeFromTransactionResults(rows) {
  return rows.map(function(v) {
    return '@' + v['valid_duration_seconds'] + ',' + v['max_fee'];
  });
}

//
// TESTS
//

test('DB integration test - transactions.reqToSql - no query string - 3 txn 9 xfer', async () => {
  let sql = transactions.reqToSql({query: {}});
  let res = await integrationDbOps.runSqlQuery(sql.query, sql.params);
  expect(res.rowCount).toEqual(9);
  expect(mapTransactionResults(res.rows).sort()).toEqual([
    '@1050: account 10 \u0127-11',
    '@1050: account 2 \u01271',
    '@1050: account 9 \u012710',
    '@1051: account 10 \u0127-21',
    '@1051: account 2 \u01271',
    '@1051: account 9 \u012720',
    '@1052: account 2 \u01271',
    '@1052: account 8 \u0127-31',
    '@1052: account 9 \u012730'
  ]);
});

test('DB integration test - transactions.reqToSql - single valid account - 1 txn 3 xfer', async () => {
  let sql = transactions.reqToSql({query: {'account.id': `${shard}.${realm}.8`}});
  let res = await integrationDbOps.runSqlQuery(sql.query, sql.params);
  expect(res.rowCount).toEqual(3);
  expect(mapTransactionResults(res.rows).sort()).toEqual([
    '@1052: account 2 \u01271',
    '@1052: account 8 \u0127-31',
    '@1052: account 9 \u012730'
  ]);
});

test('DB integration test - transactions.reqToSql - invalid account', async () => {
  let sql = transactions.reqToSql({query: {'account.id': '0.17.666'}});
  let res = await integrationDbOps.runSqlQuery(sql.query, sql.params);
  expect(res.rowCount).toEqual(0);
});

test('DB integration test - transactions.reqToSql - null validDurationSeconds and maxFee inserts', async () => {
  let res = await integrationDbOps.runSqlQuery('insert into t_record_files (name) values ($1) returning id;', [
    'nodurationfee'
  ]);
  let fileId = res.rows[0]['id'];

  await addCryptoTransferTransaction(1062, fileId, 3, 4, 50, 5, null); // null maxFee
  await addCryptoTransferTransaction(1063, fileId, 3, 4, 70, null, 777); //null validDurationSeconds
  await addCryptoTransferTransaction(1064, fileId, 3, 4, 70, null, null); // valid validDurationSeconds and maxFee

  let sql = transactions.reqToSql({query: {'account.id': '0.15.3'}});
  res = await integrationDbOps.runSqlQuery(sql.query, sql.params);
  expect(res.rowCount).toEqual(9);
  expect(extractDurationAndMaxFeeFromTransactionResults(res.rows).sort()).toEqual([
    '@5,null',
    '@5,null',
    '@5,null',
    '@null,777',
    '@null,777',
    '@null,777',
    '@null,null',
    '@null,null',
    '@null,null'
  ]);
});

test('DB integration test - transactions.reqToSql - Unknown transaction result and type', async () => {
  let res = await integrationDbOps.runSqlQuery('insert into t_record_files (name) values ($1) returning id;', [
    'unknowntypeandresult'
  ]);
  let fileId = res.rows[0]['id'];
  await addTransaction(1070, fileId, 7, [[2, 1]], 11, 33, -1, -1);

  let sql = transactions.reqToSql({query: {timestamp: '0.000001070'}});
  res = await integrationDbOps.runSqlQuery(sql.query, sql.params);
  expect(res.rowCount).toEqual(1);
  expect(res.rows[0].name).toEqual('UNKNOWN');
  expect(res.rows[0].result).toEqual('UNKNOWN');
});

const createAndPopulateNewAccount = async (id, ts, bal) => {
  await addAccount(id);
  await integrationDbOps.runSqlQuery(
    'insert into account_balances (consensus_timestamp, account_realm_num, account_num, balance) values ($1, $2, $3, $4);',
    [ts, realm, id, bal]
  );
};

test('DB integration test - transactions.reqToSql - Account range filtered transactions', async () => {
  let res = await integrationDbOps.runSqlQuery('insert into t_record_files (name) values ($1) returning id;', [
    'accountrange'
  ]);
  let fileId = res.rows[0]['id'];

  await createAndPopulateNewAccount(13, 5, 10);
  await createAndPopulateNewAccount(63, 6, 50);
  await createAndPopulateNewAccount(82, 7, 100);

  // create 3 transactions - 9 transfers
  await addCryptoTransferTransaction(2062, fileId, 13, 63, 50, 5000, 50);
  await addCryptoTransferTransaction(2063, fileId, 63, 82, 70, 7000, 777);
  await addCryptoTransferTransaction(2064, fileId, 82, 63, 20, 8000, -80);

  let sql = transactions.reqToSql({query: {'account.id': ['gte:0.15.70', 'lte:0.15.100']}});
  res = await integrationDbOps.runSqlQuery(sql.query, sql.params);

  // 6 transfers are applicable. For each transfer negative amount from self, amount to recipient and fee to bank
  // Note bank is out of desired range but is expected in query result
  expect(res.rowCount).toEqual(6);
  expect(mapTransactionResults(res.rows).sort()).toEqual([
    '@2063: account 2 \u01271',
    '@2063: account 63 \u0127-71',
    '@2063: account 82 \u012770',
    '@2064: account 2 \u01271',
    '@2064: account 63 \u012720',
    '@2064: account 82 \u0127-21'
  ]);
});

let specPath = path.join(__dirname, 'specs');
fs.readdirSync(specPath).forEach(function(file) {
  let p = path.join(specPath, file);
  let specText = fs.readFileSync(p, 'utf8');
  var spec = JSON.parse(specText);
  test(`DB integration test - ${file} - ${spec.url}`, async () => {
    // await specSetupSteps(spec);
    let response = await request(server).get(spec.url);
    expect(response.status).toEqual(spec.responseStatus);
    expect(JSON.parse(response.text)).toEqual(spec.responseJson);
  });
});

const specSetupSteps = async function(spec) {
  let recordFile = spec.fileName;
  await integrationDbOps.setUp(recordFile);

  await integrationDbOps.loadAccounts(spec.setup.accounts);
  await integrationDbOps.loadBalances(spec.setup.balances);
  await integrationDbOps.loadCryptoTransfers(spec.setup.cryptotransfers);
  await integrationDbOps.loadTransactions(spec.setup.transactions);
};
