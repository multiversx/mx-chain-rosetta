import path from "path";
import { BigUIntValue, CodeMetadata, IAddress, Interaction, ITransactionValue, ResultsParser, ReturnCode, SmartContract, SmartContractAbi, TokenPayment, Transaction, TransactionWatcher } from "@elrondnetwork/erdjs";
import { IAudit, INetworkConfig, INetworkProvider, ITestSession, ITestUser, ITokenPayment, loadAbiRegistry, loadCode } from "@elrondnetwork/erdjs-snippets";
import BigNumber from "bignumber.js";

const PathToWasm = path.resolve(__dirname, "adder.wasm");
const PathToAbi = path.resolve(__dirname, "adder.abi.json");

export async function createAdderInteractor(session: ITestSession, contractAddress?: IAddress): Promise<AdderInteractor> {
    const registry = await loadAbiRegistry(PathToAbi);
    const abi = new SmartContractAbi(registry);
    const contract = new SmartContract({ address: contractAddress, abi: abi });
    const networkProvider = session.networkProvider;
    const networkConfig = session.getNetworkConfig();
    const audit = session.audit;
    const interactor = new AdderInteractor(contract, networkProvider, networkConfig, audit);
    return interactor;
}

export class AdderInteractor {
    private readonly contract: SmartContract;
    private readonly networkProvider: INetworkProvider;
    private readonly networkConfig: INetworkConfig;
    private readonly transactionWatcher: TransactionWatcher;
    private readonly resultsParser: ResultsParser;
    private readonly audit: IAudit;

    constructor(contract: SmartContract, networkProvider: INetworkProvider, networkConfig: INetworkConfig, audit: IAudit) {
        this.contract = contract;
        this.networkProvider = networkProvider;
        this.networkConfig = networkConfig;
        this.transactionWatcher = new TransactionWatcher(networkProvider);
        this.resultsParser = new ResultsParser();
        this.audit = audit;
    }

    async deploy(deployer: ITestUser, initialValue: number): Promise<{ address: IAddress, returnCode: ReturnCode }> {
        // Load the bytecode.
        let code = await loadCode(PathToWasm);

        // Prepare the deploy transaction.
        let transaction = this.contract.deploy({
            code: code,
            codeMetadata: new CodeMetadata(),
            initArguments: [new BigUIntValue(initialValue)],
            gasLimit: 20000000,
            chainID: this.networkConfig.ChainID
        });

        // Set the transaction nonce. The account nonce must be synchronized beforehand.
        // Also, locally increment the nonce of the deployer (optional).
        transaction.setNonce(deployer.account.getNonceThenIncrement());

        // Let's sign the transaction. For dApps, use a wallet provider instead.
        await deployer.signer.sign(transaction);

        // The contract address is deterministically computable:
        const address = SmartContract.computeAddress(transaction.getSender(), transaction.getNonce());

        // Let's broadcast the transaction and await its completion:
        const transactionHash = await this.networkProvider.sendTransaction(transaction);
        await this.audit.onContractDeploymentSent({ transactionHash: transactionHash, contractAddress: address });

        let transactionOnNetwork = await this.transactionWatcher.awaitCompleted(transaction);
        await this.audit.onTransactionCompleted({ transactionHash: transactionHash, transaction: transactionOnNetwork });

        // In the end, parse the results:
        const { returnCode } = this.resultsParser.parseUntypedOutcome(transactionOnNetwork);

        console.log(`AdderInteractor.deploy(): contract = ${address}`);
        return { address, returnCode };
    }

    async add(caller: ITestUser, addValue: number): Promise<ReturnCode> {
        const transaction = await this.buildTransactionAdd(caller, addValue, 15000000);

        // Let's broadcast the transaction and await its completion:
        const transactionHash = await this.networkProvider.sendTransaction(transaction);
        console.log("add()", transactionHash);
        await this.audit.onTransactionSent({ action: "add", args: [addValue], transactionHash: transactionHash });

        let transactionOnNetwork = await this.transactionWatcher.awaitCompleted(transaction);
        await this.audit.onTransactionCompleted({ transactionHash: transactionHash, transaction: transactionOnNetwork });

        // In the end, parse the results:
        const endpoint = this.contract.getEndpoint("add");
        let { returnCode } = this.resultsParser.parseOutcome(transactionOnNetwork, endpoint);
        return returnCode;
    }

    async buildTransactionAdd(caller: ITestUser, addValue: BigNumber.Value, gasLimit: number): Promise<Transaction> {
        // Prepare the interaction
        let interaction = <Interaction>this.contract.methods
            .add([addValue])
            .withGasLimit(gasLimit)
            .withNonce(caller.account.getNonceThenIncrement())
            .withChainID(this.networkConfig.ChainID);

        // Let's check the interaction, then build the transaction object.
        let transaction = interaction.buildTransaction();

        // Let's sign the transaction. For dApps, use a wallet provider instead.
        await caller.signer.sign(transaction);
        return transaction;
    }

    async buildTransactionTransferAdd(caller: ITestUser, addValue: number, gasLimit: number, payment: TokenPayment): Promise<Transaction> {
        // Prepare the interaction
        let interaction = <Interaction>this.contract.methods
            .add([addValue])
            .withGasLimit(gasLimit)
            .withSingleESDTTransfer(payment)
            .withNonce(caller.account.getNonceThenIncrement())
            .withChainID(this.networkConfig.ChainID);

        // Let's check the interaction, then build the transaction object.
        let transaction = interaction.buildTransaction();

        // Let's sign the transaction. For dApps, use a wallet provider instead.
        await caller.signer.sign(transaction);
        return transaction;
    }

    async getSum(): Promise<number> {
        // Prepare the interaction, check it, then build the query:
        let interaction = <Interaction>this.contract.methods.getSum();
        let query = interaction.check().buildQuery();

        // Let's run the query and parse the results:
        let queryResponse = await this.networkProvider.queryContract(query);
        let { firstValue } = this.resultsParser.parseQueryResponse(queryResponse, interaction.getEndpoint());

        // Now let's interpret the results.
        let firstValueAsBigUInt = <BigUIntValue>firstValue;
        return firstValueAsBigUInt.valueOf().toNumber();
    }
}
