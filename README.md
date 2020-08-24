# dedecms-checker
input: site path

if there is site.yml under the path, check meta keywords, meta descriptions, og:image, title, baidu trackID according to the file.

checking the following:

- typedir: no /a/
- type:  keyword, description match with site
- site:
    - saving dir /
    - host according site.yml    
- css: /skin/css
- js:  /skin/js
- list-article: tkd,og:image, bds.js and their order
- index: tkd,og:image, bds.js and their order
- article-article: tkd,og:image, bds.js and their order
- bds.js exists under /skin/js and contains the trackId in site.yml
- logo.png exists /skin/images/logo.png and its dimension is 120x75
- defaultpic.gif at /skin/images
