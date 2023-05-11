# pizza
This is an engine that sources git commits and turns them to insights. Ideally this will be useful for any open source maintainer trying to get insight into their open source. 

<img width="1350" alt="Screen Shot 2023-05-11 at 8 46 18 AM" src="https://github.com/open-sauced/pizza/assets/5713670/b91989d8-df6d-4631-8d7d-3089b76ee113">

## Scope
Build a docker container clones a repo and stores the contributors and their contributions (commits) in a relational tables. This data should be store in a postgres database (😃). 

## Bonus
- Make this work with orchestration that fetches the latest data on a cron every hour.
- Add a queue to assist in fetch content without rate limiting.
- Add the ability to fetch all repos in an org.
- Visualize this data somehow.

## Gotchas
- Large repos like k8s or linux will trip `git clone` the rate limiter if done multiple times in an hour. How would you account for fetching large repos with lots of data?
