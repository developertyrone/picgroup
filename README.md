# picgroup
Stupid small GoLang program to grouping the image automatically by YearMonth or 

# TODO
1. run benchmark library [half]
   1. https://blog.logrocket.com/benchmarking-golang-improve-function-performance/
   2. https://geektutu.com/post/hpg-benchmark.html
   3. https://hackernoon.com/how-to-write-benchmarks-in-golang-like-an-expert-0w1834gs
   4. https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go
2. read files in current directory [done]
3. read files in subsequent directories [done]
4. put file entries in to map [done]
5. making the file entries into faster query data structures [done]
6. read exif data [done]
7. generate folder according to settings and files entries [done]
8. move files to correspondent folder 
9. make things concurrency mode (if possible) [done]
10. support more file format 
11. fuzzy file grouping 
12. Read number of cpu to concurrency [done]
13. Enhance to integrate with golang cli framework
14. compile to different os [done]
    1. go tool dist list
    2. https://www.digitalocean.com/community/tutorials/building-go-applications-for-different-operating-systems-and-architectures
    3. GOOS=windows GOARCH=amd64 go build -o ./bin/windows/picgroup.exe
    4. GOOS=linux GOARCH=amd64 go build -o ./bin/linux/picgroup
    5. go build -o ./bin/macm1/picgroup


# Cheatsheet

