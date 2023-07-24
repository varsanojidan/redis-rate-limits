# Redis Rate Limit PoC

Just playing around with the redis go cli, and seeing what functionality is available for distributed rate limiting.

Currently, uses a Redis + a lua script to create Token Bucket rate limiting.
