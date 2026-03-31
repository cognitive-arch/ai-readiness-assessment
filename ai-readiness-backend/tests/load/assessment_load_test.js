// tests/load/assessment_load_test.js
// k6 load test for the AI Readiness Assessment API.
//
// Install: https://k6.io/docs/getting-started/installation/
// Run:     k6 run tests/load/assessment_load_test.js
// Run with env: k6 run -e BASE_URL=http://localhost:8080 tests/load/assessment_load_test.js
//
// Scenarios:
//   browse   — lightweight GETs (questions, health)       30% of VUs
//   complete — full assessment lifecycle (create→answer→compute→pdf) 70%
//
// Thresholds (SLOs):
//   http_req_failed < 1%
//   http_req_duration p(95) < 500ms
//   http_req_duration p(99) < 2000ms

import http from "k6/http";
import { check, group, sleep } from "k6";
import { Rate, Trend } from "k6/metrics";

// ─────────────────────────────────────────────
// Configuration
// ─────────────────────────────────────────────

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

export const options = {
  scenarios: {
    browse: {
      executor: "constant-vus",
      vus: 10,
      duration: "2m",
      exec: "browse",
      tags: { scenario: "browse" },
    },
    complete: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 5 },   // ramp up
        { duration: "1m",  target: 20 },  // hold
        { duration: "30s", target: 0 },   // ramp down
      ],
      exec: "completeLifecycle",
      tags: { scenario: "complete" },
    },
  },

  thresholds: {
    http_req_failed:                     ["rate<0.01"],      // <1% errors
    "http_req_duration{scenario:browse}": ["p(95)<200"],     // browse: p95 <200ms
    "http_req_duration{scenario:complete}":["p(95)<1000"],   // compute: p95 <1s
    http_req_duration:                   ["p(99)<2000"],     // overall p99 <2s
    assessment_create_duration:          ["p(95)<300"],
    assessment_compute_duration:         ["p(95)<800"],
  },
};

// Custom metrics
const assessmentCreateDuration = new Trend("assessment_create_duration");
const assessmentComputeDuration = new Trend("assessment_compute_duration");
const lifecycleSuccess = new Rate("lifecycle_success_rate");

// ─────────────────────────────────────────────
// Scenario: browse (lightweight reads)
// ─────────────────────────────────────────────

export function browse() {
  group("health check", () => {
    const res = http.get(`${BASE_URL}/health`);
    check(res, { "health: status 200": (r) => r.status === 200 });
  });

  sleep(0.5);

  group("get question bank", () => {
    const res = http.get(`${BASE_URL}/api/questions`);
    check(res, {
      "questions: status 200":     (r) => r.status === 200,
      "questions: has domains":    (r) => r.json("data.domains") !== null,
      "questions: 72 questions":   (r) => r.json("data.questions.length") === 72,
    });
  });

  sleep(1);
}

// ─────────────────────────────────────────────
// Scenario: full assessment lifecycle
// ─────────────────────────────────────────────

export function completeLifecycle() {
  const headers = { "Content-Type": "application/json" };
  let assessmentId = null;
  let success = true;

  // 1. Create assessment
  group("create assessment", () => {
    const start = Date.now();
    const res = http.post(
      `${BASE_URL}/api/assessment`,
      JSON.stringify({ client_ref: `load-test-${__VU}-${__ITER}` }),
      { headers }
    );
    assessmentCreateDuration.add(Date.now() - start);

    const ok = check(res, {
      "create: status 201":     (r) => r.status === 201,
      "create: has id":         (r) => r.json("data.assessmentId") !== "",
    });
    if (!ok) { success = false; return; }
    assessmentId = res.json("data.assessmentId");
  });

  if (!assessmentId) {
    lifecycleSuccess.add(false);
    return;
  }

  sleep(0.2);

  // 2. Save answers in 3 batches (simulating domain-by-domain completion)
  const domainBatches = [
    buildDomainAnswers(["s1","s2","s3","s4","s5","s6","s7","s8","s9","s10","s11","s12"]),
    buildDomainAnswers(["t1","t2","t3","t4","t5","t6","t7","t8","t9","t10","t11","t12",
                        "d1","d2","d3","d4","d5","d6","d7","d8","d9","d10","d11","d12"]),
    buildDomainAnswers(["o1","o2","o3","o4","o5","o6","o7","o8","o9","o10","o11","o12",
                        "sec1","sec2","sec3","sec4","sec5","sec6","sec7","sec8","sec9","sec10","sec11","sec12",
                        "u1","u2","u3","u4","u5","u6","u7","u8","u9","u10","u11","u12"]),
  ];

  for (let i = 0; i < domainBatches.length; i++) {
    group(`save answers batch ${i + 1}`, () => {
      const res = http.put(
        `${BASE_URL}/api/assessment/${assessmentId}/answers`,
        JSON.stringify({ answers: domainBatches[i] }),
        { headers }
      );
      const ok = check(res, {
        [`batch ${i + 1}: status 200`]: (r) => r.status === 200,
      });
      if (!ok) success = false;
    });
    sleep(0.3);
  }

  // 3. Compute results
  group("compute", () => {
    const start = Date.now();
    const res = http.post(
      `${BASE_URL}/api/assessment/${assessmentId}/compute`,
      "",
      { headers }
    );
    assessmentComputeDuration.add(Date.now() - start);

    const ok = check(res, {
      "compute: status 200":    (r) => r.status === 200,
      "compute: has result":    (r) => r.json("data.result.overall") !== null,
      "compute: valid maturity":(r) => [
        "Foundational Risk Zone","AI Emerging","AI Structured","AI Advanced","AI-Native"
      ].includes(r.json("data.result.maturity")),
    });
    if (!ok) success = false;
  });

  sleep(0.2);

  // 4. Get results
  group("get results", () => {
    const res = http.get(`${BASE_URL}/api/assessment/${assessmentId}/results`);
    check(res, {
      "results: status 200": (r) => r.status === 200,
      "results: has overall": (r) => typeof r.json("data.overall") === "number",
    });
  });

  sleep(0.1);

  // 5. Export PDF (sampled — only 20% of VUs to avoid I/O saturation)
  if (__VU % 5 === 0) {
    group("export pdf", () => {
      const res = http.get(`${BASE_URL}/api/assessment/${assessmentId}/export/pdf`);
      check(res, {
        "pdf: status 200":       (r) => r.status === 200,
        "pdf: content-type":     (r) => r.headers["Content-Type"] === "application/pdf",
        "pdf: non-empty body":   (r) => r.body.length > 100,
      });
    });
    sleep(0.2);
  }

  // 6. Cleanup — delete the test assessment
  group("delete", () => {
    const res = http.del(`${BASE_URL}/api/assessment/${assessmentId}`);
    check(res, { "delete: status 200": (r) => r.status === 200 });
  });

  lifecycleSuccess.add(success);
  sleep(1);
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

function buildDomainAnswers(qids) {
  const answers = {};
  qids.forEach((qid, i) => {
    answers[qid] = {
      score: (i % 5) + 1,  // cycles through 1-5
      comment: i % 3 === 0 ? `Load test note for ${qid}` : "",
    };
  });
  return answers;
}
