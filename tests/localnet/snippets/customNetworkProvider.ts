import { ProxyNetworkProvider } from "@elrondnetwork/erdjs-network-providers";
import { ITransaction } from "@elrondnetwork/erdjs-snippets";

export class CustomNetworkProvider extends ProxyNetworkProvider {
    constructor() {
        super("http://localhost:7950")
    }

    async sendTransactions(transactions: ITransaction[]): Promise<void> {
        const sendableTransactions = transactions.map(item => item.toSendable());
        const response = await this.doPostGeneric("transaction/send-multiple", sendableTransactions);
        console.log("Sent transactions:", response.numOfSentTxs);
    }
}
