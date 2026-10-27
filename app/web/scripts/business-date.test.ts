import { test } from "node:test";
import assert from "node:assert/strict";
import { businessDate } from "../src/lib/business-date.ts";

test("Shanghai midnight while Los Angeles is on yesterday", () => {
  assert.equal(businessDate(new Date("2026-10-26T15:59:59Z")), "2026-10-26");
  assert.equal(businessDate(new Date("2026-10-26T16:00:00Z")), "2026-10-27");
});
test("month and year boundaries use the business calendar", () => {
  assert.equal(businessDate(new Date("2026-12-31T16:01:00Z")), "2027-01-01");
  assert.equal(businessDate(new Date("2026-02-28T16:01:00Z")), "2026-03-01");
});
test("US daylight saving does not change the business day", () => {
  assert.equal(businessDate(new Date("2026-03-08T07:59:00Z")), "2026-03-08");
  assert.equal(businessDate(new Date("2026-03-08T10:01:00Z")), "2026-03-08");
});
