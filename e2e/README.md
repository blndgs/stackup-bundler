# Bundler E2E tests

A repeatable set of E2E tests to automate QA checks for the bundler. This should be used in addition to the [bundler test suite](https://github.com/eth-infinitism/bundler-spec-tests).

# Usage

Below are instructions on how to run a series of E2E tests to check that everything is working as expected. The tests will execute a collection of known transactions that cover a wide range of edge cases.

## Prerequisites

The steps in the following section assumes that all these tools have been installed and ready to go.

- Node.JS = 18
- [Geth](tested with 1.13.5 https://geth.ethereum.org/docs/getting-started/installing-geth)

## Setting the environment

To reduce the impact of external factors, we'll run the E2E test using an isolated local instance of both geth and the bundler.

First, in a new tab/pane run a local instance of geth with the following command:

```bash
geth \
  --http.vhosts '*,localhost,host.docker.internal' \
  --http \
  --http.api eth,net,web3,debug \
  --http.corsdomain '*' \
  --http.addr "0.0.0.0" \
  --nodiscover --maxpeers 0 --mine \
  --networkid 1337 \
  --dev \
  --allow-insecure-unlock \
  --rpc.allow-unprotected-txs \
  --miner.gaslimit 12000000
```

In a separate process,
- Clone [eth-infinitism/account-abstraction](https://github.com/eth-infinitism/account-abstraction/)
- Checkout the latest tag that is live on mainnet, currently v0.6.0
- ```yarn install```
- Run the following command to deploy the required contracts:

```bash
yarn deploy --network localhost
```

- Navigate to the [stackup-wallet/contracts](https://github.com/stackup-wallet/contracts) directory.
- ```yarn install```
- Run the following command to deploy the supporting test contracts:

```bash
yarn deploy:AllTest --network localhost
```

- In a new pane/tab `cd ../` to [github.com/blndgs/stackup-bundler](https://github.com/blndgs/stackup-bundler)
- Set the following environment variables:

```
ERC4337_BUNDLER_ETH_CLIENT_URL=http://localhost:8545
ERC4337_BUNDLER_PRIVATE_KEY=c6cbc5ffad570fdad0544d1b5358a36edeb98d163b6567912ac4754e144d4edb
ERC4337_BUNDLER_MAX_BATCH_GAS_LIMIT=12000000
ERC4337_BUNDLER_DEBUG_MODE=true
```

Run the bundler with the following config:
`make dev-private-mode`

## Running the test suite

- Navigate to stackup-wallet/e2e directory (`cd ./e2e`)
- ```yarn install```
- check the contents of the `config.json` file to ensure the `private key` matches the `ERC4337_BUNDLER_PRIVATE_KEY` environment variable set above.
- Use the following command to run the eth_infinitism test suite.

```bash
yarn run test # see note below if you get a sender_address error
```

** _Note: try the following step_
edit the `setup.ts` file and hardcode the `config.ts` contents into a config object. This is a temporary workaround until a solution for reading `config.ts`.

```typescript
import { fundIfRequired } from "./src/helpers";
// import config from "./config";

const config = {
    // This is for testing only. DO NOT use in production.
    // signingKey: '19b8ac9d574d2dcddc3b9f3ae29aec0bb1f13519c8eed4ff8ddcee642076d689',
    signingKey:
        "c6cbc5ffad570fdad0544d1b5358a36edeb98d163b6567912ac4754e144d4edb",
    nodeUrl: "http://localhost:8545",
    bundlerUrl: "http://localhost:4337",

    // https://github.com/stackup-wallet/contracts/blob/main/contracts/test
    testERC20Token: "0x3870419Ba2BBf0127060bCB37f69A1b1C090992B",
    testGas: "0xc2e76Ee793a194Dd930C18c4cDeC93E7C75d567C",
    testAccount: "0x3dFD39F2c17625b301ae0EF72B411D1de5211325",
};

// no changes below this line
export default async function () {
    // ...
```

try again with `yarn run test`