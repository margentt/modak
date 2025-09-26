# Rate-Limited Notification Service

## Problem Statement
We have a Notification system that sends out email notifications of various types (status update, daily news, project invitations, etc). We need to protect recipients from getting too many emails, either due to system errors or due to abuse, so let’s limit the number of emails sent to them by implementing a rate-limited version of NotificationService.

The system must reject requests that are over the limit.

Some sample notification types and rate limit rules, e.g.:

Status: not more than 2 per minute for each recipient

News: not more than 1 per day for each recipient

Marketing: not more than 3 per hour for each recipient

Etc. these are just samples, the system might have several rate limit rules!

## Implementation

The in-memory rate limiter is implemented using a sliding window log algorithm, which tracks the number of notifications sent per user and per notification type.

For each incoming notification:

- We retrieve the list of prior timestamps for the (user, type) pair.
- Using a binary search, we prune timestamps older than the interval defined in the rate-limit rule.
- If the remaining list reaches the limit, the notification is blocked. Otherwise, we add the new timestamp and allow it.

Concurrent requests are processed sequentially using a mutex associated with each (user, type) key.

Memory usage is bounded by the number of active keys (i.e. notifications with defined rules) times the limit for each rule. The number of active keys is at most (number of users * number of rules). The cache automatically evicts inactive entries and their mutexes.

I decided not to use alternatives such as the [sliding window counter](https://www.figma.com/blog/an-alternative-approach-to-rate-limiting/#sliding-window-counters) or leaky bucket. In the context of a per-user rate limiter with such low limits, buckets would often contain only one or a few events, resulting in a similar memory usage as storing individual timestamps, but with reduced accuracy for sudden bursts.

## Limitations and improvements

### Single Rule per notification Type
The in-memory approach currently supports only one rule per notification type. To handle multiple rules, we could store all timestamps starting from the maximum interval and,  for each interval, use a binary search to count events and check against the limit.

### Distributed service
Since this rate limiter is in-memory, it is not suitable for a distributed system with multiple tasks or instances. One way to handle this is to implement a similar sliding-window algorithm in Redis, using Lua scripts to ensure atomic operations on concurrent requests.
