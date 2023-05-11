# pizza
This is an engine that sources git commits and turns them to insights

## Scope
Build a docker container clones a repo and stores the contributors and their contributions (commits) in a relational tables. This data should be store in a postgres database (😃). 

## Bonus
- Make this work with orchestration that fetches the latest data on a cron every hour.
- Add a queue to assist in fetch content without rate limiting.
- Add the ability to fetch all repos in an org.
- Visualize this data somehow.

## Gotchas
- Large repos like k8s or linux will trip the rate limiter. How would you account for fetching large repos with lots of data.
