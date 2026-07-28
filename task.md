# Task List - Display Student CBT Exam Scores in My Test Section

- [x] Modify Backend `test_series` Service
  - [x] Update `GetStudentAttemptForTest` in [postgres_test_series.go](file:///d:/Clasynq_future_update/API_2.0/test_series/internal/repository/postgres_test_series.go) to support `submitted` and `ongoing` statuses.
  - [x] Update `populateTestSeriesVirtualFields` in [test_series_usecase.go](file:///d:/Clasynq_future_update/API_2.0/test_series/internal/usecase/test_series_usecase.go) to extract scores for `"submitted"` status.
  - [x] Rebuild and compile `test_series` microservice.
- [x] Modify Frontend `frontend_02` Repository
  - [x] Update [MyTests.jsx](file:///d:/Clasynq_future_update/frontend_02/src/pages/MyTests.jsx) status checks and button handlers.
  - [x] Update [MyTests.jsx](file:///d:/Clasynq_future_update/frontend_02/src/pages/MyTests.jsx) to display scores on test cards.
  - [x] Verify frontend compiles cleanly using `npm run build`.
- [x] Git Commit & Deploy
  - [x] Push backend changes to test branch.
  - [x] Push frontend changes to test branch.
