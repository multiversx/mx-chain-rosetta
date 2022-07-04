import { generateSecretKeys, ISecretKeysGeneratorConfig, OneMinuteInMilliseconds } from "@elrondnetwork/erdjs-snippets";
import { PathLike, readFileSync } from "fs";

describe("user operations snippet", async function () {
    it("generate keys", async function () {
        this.timeout(OneMinuteInMilliseconds);

        const config = readJson<ISecretKeysGeneratorConfig>("usersGenerator.json");
        await generateSecretKeys(config);
    });
});

export function readJson<T>(filePath: PathLike): T {
    const json = readFileSync(filePath, { encoding: "utf8" });
    const parsed = <T>JSON.parse(json);
    return parsed;
}
