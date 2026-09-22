#!/usr/bin/env ruby
# frozen_string_literal: true

require "date"
require "json"
require "net/http"
require "nokogiri"
require "open-uri"
require "optparse"
require "tempfile"
require "uri"

module BcsImport
  ARCHIVE_URL = "https://beneath-ceaseless-skies.com/issues/2026/"
  USER_AGENT = "LinkScope BCS importer/1.0"
  MAX_AUDIO_BYTES = 100 << 20

  module_function

  # 归档页的每一期混有播客和往期推荐；只取子标题出现前的前两篇正式文章。
  def parse_archive(html)
    document = Nokogiri::HTML(html)
    document.css(".post-title").flat_map do |heading|
      next [] unless heading.text.include?("Issue #")

      body = heading.next_element
      next [] unless body&.matches?(".post-body")

      date_text = heading.at_css(".post-date")&.text.to_s.strip
      published_at = Date.parse(date_text).strftime("%Y-%m-%d")
      cover_url = body.at_css(".cover-image img")&.[]("src").to_s
      current_stories = []

      body.css(".issue-story-title").each do |story_node|
        # 任一子标题后的内容都不是本期正式文章。
        break if story_node.xpath("preceding-sibling::*[contains(concat(' ', normalize-space(@class), ' '), ' issue-podcast-subheader ')]").any?

        link = story_node.at_css("a[href*='/stories/']")
        next unless link

        current_stories << {
          title: clean_text(link.text),
          slug: URI(link["href"]).path.split("/").reject(&:empty?).last,
          source_url: link["href"],
          excerpt: excerpt_after(story_node),
          published_at: published_at,
          cover_url: cover_url
        }
        break if current_stories.length == 2
      end
      current_stories
    end.compact
  end

  def podcast_title_key(title)
    clean_text(title).downcase.gsub(/[^\p{L}\p{N}]+/u, " ").strip
  end

  def parse_podcasts(html, unmatched: [])
    document = Nokogiri::HTML(html)
    document.css(".post-body").flat_map do |body|
      formal_stories = {}
      body.css(".issue-story-title").each do |story_node|
        break if story_node.xpath("preceding-sibling::*[contains(concat(' ', normalize-space(@class), ' '), ' issue-podcast-subheader ')]").any?

        link = story_node.at_css("a[href*='/stories/']")
        next unless link

        slug = URI(link["href"]).path.split("/").reject(&:empty?).last
        formal_stories[podcast_title_key(link.text)] = slug
      end

      body.css(".issue-podcast-subheader").map do |subheader|
        next unless clean_text(subheader.text).include?("Audio Fiction Podcast")

        title_node = subheader.next_element
        next unless title_node&.matches?(".issue-story-title")

        title = clean_text(title_node.text)
        slug = formal_stories[podcast_title_key(title)]
        unless slug
          unmatched << title
          next
        end

        source = nil
        details = ""
        node = title_node.next_element
        while node && !node.matches?(".issue-podcast-subheader")
          source ||= node.at_css("audio source[type='audio/mpeg']")&.[]("src")
          details = clean_text(node.text) if node.name == "p" && node.text.include?("Duration:")
          node = node.next_element
        end
        match = details.match(/Duration:\s*([0-9:]+)\s*[—-]\s*([0-9.]+\s*[KMG]?B)/i)
        next unless source && match

        uri = URI(source)
        uri.query = nil
        { slug: slug, audio_url: uri.to_s, duration: match[1], declared_size: match[2].delete(" ") }
      end.compact
    end
  end

  def excerpt_after(story_node)
    excerpts = []
    node = story_node.next_element
    while node && !node.matches?(".issue-story-title, .issue-podcast-subheader")
      excerpts << node if node.matches?(".issue-story-excerpt")
      node = node.next_element
    end
    preferred = excerpts.find { |item| item["class"].to_s.split.include?("short") } || excerpts.first
    truncate(clean_text(preferred&.text.to_s), 500)
  end

  # 只转换正文容器，作者简介、评论和推荐文章都在容器外，不会被带入。
  def parse_story_body(html)
    container = Nokogiri::HTML(html).at_css(".bcs-story-content")
    raise "详情页缺少 .bcs-story-content" unless container

    # 使用 map + compact 兼容项目电脑自带的 Ruby 2.6。
    blocks = container.element_children.map do |node|
      case node.name
      when "p"
        normalize_markdown(render_inline(node))
      when "div"
        "---" if node["class"].to_s.split.any? { |name| name == "section_break" || name == "section-break" }
      when "blockquote"
        clean_text(node.text).split("\n").map { |line| "> #{line}" }.join("\n")
      when "h1", "h2", "h3", "h4", "h5", "h6"
        "#{'#' * node.name.delete_prefix('h').to_i} #{clean_text(node.text)}"
      end
    end.compact
    markdown = blocks.reject(&:empty?).join("\n\n").strip
    raise "详情页正文为空" if markdown.empty?

    markdown
  end

  def render_inline(node)
    node.children.map do |child|
      if child.text?
        child.text
      elsif child.name == "br"
        "\n"
      elsif %w[em i].include?(child.name)
        "*#{render_inline(child).strip}*"
      elsif %w[strong b].include?(child.name)
        "**#{render_inline(child).strip}**"
      else
        render_inline(child)
      end
    end.join
  end

  def normalize_markdown(text)
    text.gsub("\u00A0", " ").gsub(/[ \t\r\f]+/, " ").gsub(/ *\n */, "\n").strip
  end

  def clean_text(text)
    text.gsub("\u00A0", " ").gsub(/\s+/, " ").strip
  end

  def truncate(text, maximum)
    return text if text.length <= maximum

    "#{text[0, maximum - 3].rstrip}..."
  end

  def pending_stories(stories, existing_slugs)
    existing = existing_slugs.each_with_object({}) { |slug, result| result[slug] = true }
    stories.reject { |story| existing[story.fetch(:slug)] }
  end

  def fetch(url)
    URI.open(url, "User-Agent" => USER_AGENT, open_timeout: 15, read_timeout: 45, &:read)
  end

  def crawl
    stories = []
    next_url = ARCHIVE_URL
    visited = {}
    while next_url && !visited[next_url]
      visited[next_url] = true
      html = fetch(next_url)
      document = Nokogiri::HTML(html)
      stories.concat(parse_archive(html))
      next_link = document.at_xpath("//a[contains(normalize-space(.), 'Next Page')]")
      next_url = next_link && URI.join(next_url, next_link["href"]).to_s
    end

    stories.each do |story|
      story[:body_markdown] = parse_story_body(fetch(story.fetch(:source_url)))
    end
    stories
  end

  def crawl_podcasts
    podcasts = []
    unmatched = []
    next_url = ARCHIVE_URL
    visited = {}
    while next_url && !visited[next_url]
      visited[next_url] = true
      html = fetch(next_url)
      document = Nokogiri::HTML(html)
      podcasts.concat(parse_podcasts(html, unmatched: unmatched))
      next_link = document.at_xpath("//a[contains(normalize-space(.), 'Next Page')]")
      next_url = next_link && URI.join(next_url, next_link["href"]).to_s
    end
    [podcasts, unmatched]
  end

  def mp3_header?(data)
    data.start_with?("ID3") || (data.bytesize >= 2 && data.getbyte(0) == 0xff && (data.getbyte(1) & 0xe0) == 0xe0)
  end

  def with_downloaded_audio(url, max_bytes: MAX_AUDIO_BYTES)
    uri = URI(url)
    4.times do
      redirected = nil
      Tempfile.create(["bcs-audio", ".mp3"]) do |file|
        file.binmode
        http = Net::HTTP.new(uri.host, uri.port)
        http.use_ssl = uri.scheme == "https"
        http.open_timeout = 15
        http.read_timeout = 90
        request = Net::HTTP::Get.new(uri)
        request["User-Agent"] = USER_AGENT
        http.request(request) do |response|
          if response.is_a?(Net::HTTPRedirection)
            redirected = URI.join(uri, response.fetch("location"))
            next
          end
          raise "下载失败：HTTP #{response.code}" unless response.is_a?(Net::HTTPSuccess)
          if response["content-length"].to_i > max_bytes
            raise "音频超过 #{max_bytes} 字节"
          end

          size = 0
          response.read_body do |chunk|
            size += chunk.bytesize
            raise "音频超过 #{max_bytes} 字节" if size > max_bytes

            file.write(chunk)
          end
          file.flush
          file.rewind
          header = file.read(3).to_s
          raise "下载内容不是有效 MP3" unless mp3_header?(header)

          file.rewind
          yield file, size
          return
        end
      end
      if redirected
        uri = redirected
        next
      end
      break
    end
    raise "音频重定向次数过多"
  end

  def editable_story_payload(story)
    %w[title slug category excerpt body_markdown cover_path audio_path audio_duration audio_size_bytes published_at enabled featured].each_with_object({}) do |key, payload|
      payload[key] = story.fetch(key)
    end
  end

  def import_podcasts(podcasts:, stories:, client:, apply:, refresh_audio:, max_audio_bytes: MAX_AUDIO_BYTES)
    result = { eligible: [], attached: [], skipped: [], failed: [] }
    podcasts.each_with_index do |podcast, index|
      slug = podcast.fetch(:slug)
      story = stories[slug]
      unless story
        result[:failed] << { slug: slug, error: "后台不存在同 slug 正式文章" }
        next
      end
      if !refresh_audio && !story.fetch("audio_path", "").empty?
        result[:skipped] << slug
        puts "[#{index + 1}/#{podcasts.length}] 跳过 #{slug}：已有音频"
        next
      end

      begin
        with_downloaded_audio(podcast.fetch(:audio_url), max_bytes: max_audio_bytes) do |file, size|
          result[:eligible] << slug
          if apply
            uploaded = client.upload_audio(file)
            payload = editable_story_payload(story)
            payload["audio_path"] = uploaded.fetch("path")
            payload["audio_duration"] = podcast.fetch(:duration)
            payload["audio_size_bytes"] = uploaded.fetch("size_bytes", size)
            client.update_story(story.fetch("id"), payload)
            result[:attached] << slug
          end
        end
        puts "[#{index + 1}/#{podcasts.length}] #{apply ? '已关联' : '验证通过'} #{slug}"
      rescue StandardError => error
        result[:failed] << { slug: slug, error: error.message }
        warn "[#{index + 1}/#{podcasts.length}] 失败 #{slug}：#{error.message}"
      end
    end
    result
  end

  # 仅从项目 .env 读取本地管理凭据，任何日志都不会输出密码。
  def load_env(path)
    return {} unless File.exist?(path)

    File.readlines(path, chomp: true).each_with_object({}) do |line, values|
      next if line.strip.empty? || line.lstrip.start_with?("#") || !line.include?("=")

      key, value = line.split("=", 2)
      values[key.strip] = value.strip.sub(/\A['\"]/, "").sub(/['\"]\z/, "")
    end
  end

  class AdminClient
    def initialize(base_url:, username:, password:)
      @base_url = base_url.sub(%r{/+$}, "")
      @username = username
      @password = password
      @cookie = nil
    end

    def login
      response = request(:post, "/api/v1/auth/login", { username: @username, password: @password })
      @cookie = response.fetch("set-cookie").split(";", 2).first
    end

    def existing_slugs
      body = request(:get, "/api/v1/audio-novels?page=1&page_size=100").fetch("json")
      body.fetch("items").map { |item| item.fetch("slug") }
    end

    def all_stories
      items = request(:get, "/api/v1/audio-novels?page=1&page_size=100").fetch("json").fetch("items")
      items.each_with_object({}) do |summary, stories|
        item = request(:get, "/api/v1/audio-novels/#{summary.fetch('id')}").fetch("json")
        stories[item.fetch("slug")] = item
      end
    end

    def upload_cover(url)
      uri = URI(url)
      extension = File.extname(uri.path).downcase
      extension = ".jpg" unless %w[.jpg .jpeg .png .webp].include?(extension)
      Tempfile.create(["bcs-cover", extension]) do |file|
        file.binmode
        file.write(BcsImport.fetch(url))
        file.flush
        file.rewind
        response = request(:post, "/api/v1/audio-novel-covers", nil, { form: [["file", file]] })
        return response.fetch("json").fetch("path")
      end
    end

    def create_story(story, cover_path)
      payload = {
        title: story.fetch(:title),
        slug: story.fetch(:slug),
        category: "Fantasy",
        excerpt: story.fetch(:excerpt),
        body_markdown: story.fetch(:body_markdown),
        cover_path: cover_path,
        published_at: story.fetch(:published_at),
        enabled: true,
        featured: false
      }
      request(:post, "/api/v1/audio-novels", payload).fetch("json")
    end

    def upload_audio(file)
      response = request(:post, "/api/v1/audio-novel-audio", nil, { form: [["file", file]] })
      response.fetch("json")
    end

    def update_story(id, payload)
      request(:patch, "/api/v1/audio-novels/#{id}", payload).fetch("json")
    end

    private

    # options 使用普通 Hash，避免 Ruby 2.6 把 JSON payload 误判为关键字参数。
    def request(method, path, payload = nil, options = {})
      uri = URI("#{@base_url}#{path}")
      http = Net::HTTP.new(uri.host, uri.port)
      http.use_ssl = uri.scheme == "https"
      http.open_timeout = 10
      http.read_timeout = 60
      request_class = { get: Net::HTTP::Get, post: Net::HTTP::Post, patch: Net::HTTP::Patch }.fetch(method)
      request = request_class.new(uri)
      request["X-Requested-With"] = "XMLHttpRequest"
      request["Cookie"] = @cookie if @cookie
      if options[:form]
        request.set_form(options[:form], "multipart/form-data")
      elsif payload
        request["Content-Type"] = "application/json"
        request.body = JSON.generate(payload)
      end
      response = http.request(request)
      raise "#{method.to_s.upcase} #{path} 失败：HTTP #{response.code} #{response.body}" unless response.is_a?(Net::HTTPSuccess)

      parsed = response.body.to_s.empty? ? {} : JSON.parse(response.body)
      { "json" => parsed, "set-cookie" => response["set-cookie"].to_s }
    end
  end

  def run(argv)
    options = { apply: false, audio: false, refresh_audio: false, base_url: "http://127.0.0.1:8080" }
    OptionParser.new do |parser|
      parser.banner = "Usage: ruby scripts/import_bcs_2026.rb [--apply] [--audio]"
      parser.on("--apply", "预检通过后写入本地语音小说后台") { options[:apply] = true }
      parser.on("--audio", "验证并关联 2026 Audio Fiction MP3") { options[:audio] = true }
      parser.on("--refresh-audio", "重新下载并替换已有 MP3") { options[:audio] = true; options[:refresh_audio] = true }
      parser.on("--base-url URL", "管理后台地址") { |value| options[:base_url] = value }
    end.parse!(argv)

    if options[:audio]
      podcasts, unmatched = crawl_podcasts
      raise "Podcast 数量异常：期望 11 个，实际 #{podcasts.length} 个" unless podcasts.length == 11
      warn "未匹配正式文章：#{unmatched.join(', ')}" unless unmatched.empty?

      env = load_env(File.expand_path("../.env", __dir__))
      client = AdminClient.new(base_url: options[:base_url], username: env.fetch("ADMIN_USER", "admin"), password: env.fetch("ADMIN_PASSWORD"))
      client.login
      result = import_podcasts(podcasts: podcasts, stories: client.all_stories, client: client, apply: options[:apply], refresh_audio: options[:refresh_audio])
      puts "完成：可用 #{result[:eligible].length}，已关联 #{result[:attached].length}，跳过 #{result[:skipped].length}，失败 #{result[:failed].length}。"
      return
    end

    stories = crawl
    slugs = stories.map { |story| story.fetch(:slug) }
    raise "抓取数量异常：期望 38 篇，实际 #{stories.length} 篇" unless stories.length == 38
    raise "抓取结果存在重复 slug" unless slugs.uniq.length == slugs.length

    puts "预检通过：#{stories.length} 篇正式文章，正文均已读取。"
    return unless options[:apply]

    env = load_env(File.expand_path("../.env", __dir__))
    client = AdminClient.new(
      base_url: options[:base_url],
      username: env.fetch("ADMIN_USER", "admin"),
      password: env.fetch("ADMIN_PASSWORD")
    )
    client.login
    pending = pending_stories(stories, client.existing_slugs)
    puts "已有 #{stories.length - pending.length} 篇同 slug 内容，本次新增 #{pending.length} 篇。"

    # 同一期的两篇文章复用一次上传后的封面路径，避免生成重复文件。
    uploaded_covers = {}
    pending.each_with_index do |story, index|
      cover_path = uploaded_covers[story.fetch(:cover_url)] ||= client.upload_cover(story.fetch(:cover_url))
      client.create_story(story, cover_path)
      puts "[#{index + 1}/#{pending.length}] 已导入 #{story.fetch(:title)}"
    end
  end
end

BcsImport.run(ARGV) if $PROGRAM_NAME == __FILE__
