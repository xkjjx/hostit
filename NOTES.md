Flow is:

if storage already exists
- verify that connection (github pages/cloudfront is valid)
- diff files, make commit with diff
else
- create connections from scratch
- make upload of all new data


maybe separate functionality of setting up cloudfront in object storage manager even if it means that
github implementation doesn't need it
