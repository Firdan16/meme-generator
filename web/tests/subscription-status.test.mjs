import { expect, test } from "bun:test";
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { subscriptionStatusOptions } from "../src/lib/queries.ts";

test("returning to a screen refreshes a previously loaded subscription", async () => {
  const client = new QueryClient({ defaultOptions: { queries: { gcTime: Infinity } } });
  const options = subscriptionStatusOptions("fixture-user");
  const active = { subscribed: true, planName: "Premium 1 Hari", maxMemes: 10, memesUsed: 0 };
  client.setQueryData(options.queryKey, { subscribed: false, maxMemes: 0, memesUsed: 0 });
  const observer = new QueryObserver(client, { ...options, queryFn: async () => active });
  const unsubscribe = observer.subscribe(() => {});
  try {
    expect(observer.getCurrentResult().fetchStatus).toBe("fetching");
    await observer.getCurrentQuery().promise;
    expect(observer.getCurrentResult().data).toEqual(active);
  } finally {
    unsubscribe();
    client.clear();
  }
});
