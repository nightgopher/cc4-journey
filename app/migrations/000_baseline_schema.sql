--
-- PostgreSQL database dump
--

\restrict ncknEsajz3RMpZze6RZlOQfSiw2xuPASc7zKv2hFTQjA5GwOozruvRxdSG40we7

-- Dumped from database version 15.19
-- Dumped by pg_dump version 15.19

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: sync_monthly_attendance_workdays(); Type: FUNCTION; Schema: public; Owner: root
--

CREATE FUNCTION public.sync_monthly_attendance_workdays() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- 当工作日配置更新时，同步更新对应月份的所有月度考勤汇总记录
    UPDATE monthly_attendance_summary
    SET total_workdays = NEW.total_workdays,
        updated_at = CURRENT_TIMESTAMP
    WHERE year = NEW.year
      AND month = NEW.month;

    RAISE NOTICE '已同步 % 年 % 月的工作日数: % 天', NEW.year, NEW.month, NEW.total_workdays;

    RETURN NEW;
END;
$$;


ALTER FUNCTION public.sync_monthly_attendance_workdays() OWNER TO root;

--
-- Name: update_monthly_attendance_summary_updated_at(); Type: FUNCTION; Schema: public; Owner: root
--

CREATE FUNCTION public.update_monthly_attendance_summary_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.update_monthly_attendance_summary_updated_at() OWNER TO root;

--
-- Name: update_updated_at_column(); Type: FUNCTION; Schema: public; Owner: root
--

CREATE FUNCTION public.update_updated_at_column() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.update_updated_at_column() OWNER TO root;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: approval_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.approval_records (
    id integer NOT NULL,
    request_id integer NOT NULL,
    approver_id integer NOT NULL,
    action character varying(20) NOT NULL,
    comments text,
    next_approver_id integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT approval_records_action_check CHECK (((action)::text = ANY (ARRAY[('approved'::character varying)::text, ('rejected'::character varying)::text, ('forwarded'::character varying)::text])))
);


ALTER TABLE public.approval_records OWNER TO root;

--
-- Name: TABLE approval_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.approval_records IS '审批记录表 / Approval records table';


--
-- Name: COLUMN approval_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.id IS '审批记录唯一标识符 / Approval record unique identifier';


--
-- Name: COLUMN approval_records.request_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.request_id IS '关联的请求ID / Associated request ID';


--
-- Name: COLUMN approval_records.approver_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.approver_id IS '审批人ID / Approver ID';


--
-- Name: COLUMN approval_records.action; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.action IS '操作 / Action';


--
-- Name: COLUMN approval_records.comments; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.comments IS '评论 / Comments';


--
-- Name: COLUMN approval_records.next_approver_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.next_approver_id IS '下一个审批人 / Next approver';


--
-- Name: COLUMN approval_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_records.created_at IS '创建时间 / Creation time';


--
-- Name: approval_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.approval_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.approval_records_id_seq OWNER TO root;

--
-- Name: approval_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.approval_records_id_seq OWNED BY public.approval_records.id;


--
-- Name: approval_requests; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.approval_requests (
    id integer NOT NULL,
    request_type character varying(50) NOT NULL,
    title character varying(200) NOT NULL,
    description text,
    priority character varying(10) DEFAULT 'medium'::character varying,
    requested_by integer NOT NULL,
    current_approver integer,
    status character varying(20) DEFAULT 'pending'::character varying,
    metadata jsonb DEFAULT '{}'::jsonb,
    requested_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT approval_requests_priority_check CHECK (((priority)::text = ANY (ARRAY[('high'::character varying)::text, ('medium'::character varying)::text, ('low'::character varying)::text]))),
    CONSTRAINT approval_requests_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('approved'::character varying)::text, ('rejected'::character varying)::text, ('cancelled'::character varying)::text])))
);


ALTER TABLE public.approval_requests OWNER TO root;

--
-- Name: TABLE approval_requests; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.approval_requests IS '审批请求表 / Approval requests table';


--
-- Name: COLUMN approval_requests.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.id IS '审批请求唯一标识符 / Approval request unique identifier';


--
-- Name: COLUMN approval_requests.request_type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.request_type IS '请求类型 / Request type';


--
-- Name: COLUMN approval_requests.title; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.title IS '标题 / Title';


--
-- Name: COLUMN approval_requests.description; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.description IS '描述 / Description';


--
-- Name: COLUMN approval_requests.priority; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.priority IS '优先级 / Priority';


--
-- Name: COLUMN approval_requests.requested_by; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.requested_by IS '请求人 / Requested by';


--
-- Name: COLUMN approval_requests.current_approver; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.current_approver IS '当前审批人 / Current approver';


--
-- Name: COLUMN approval_requests.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.status IS '状态 / Status';


--
-- Name: COLUMN approval_requests.metadata; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.metadata IS '元数据 / Metadata';


--
-- Name: COLUMN approval_requests.requested_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.requested_at IS '请求时间 / Request time';


--
-- Name: COLUMN approval_requests.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.approval_requests.updated_at IS '更新时间 / Update time';


--
-- Name: approval_requests_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.approval_requests_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.approval_requests_id_seq OWNER TO root;

--
-- Name: approval_requests_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.approval_requests_id_seq OWNED BY public.approval_requests.id;


--
-- Name: attendance_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.attendance_records (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    date date NOT NULL,
    status character varying(20) DEFAULT 'absent'::character varying,
    leave_type character varying(20),
    attendance_days numeric(4,2) DEFAULT 1.0,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT attendance_records_status_check CHECK (((status)::text = ANY (ARRAY[('work'::character varying)::text, ('absent'::character varying)::text, ('partial'::character varying)::text])))
);


ALTER TABLE public.attendance_records OWNER TO root;

--
-- Name: TABLE attendance_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.attendance_records IS '考勤记录表 / Attendance records table';


--
-- Name: COLUMN attendance_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.id IS '考勤记录唯一标识符 / Attendance record unique identifier';


--
-- Name: COLUMN attendance_records.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN attendance_records.date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.date IS '考勤日期 / Attendance date';


--
-- Name: COLUMN attendance_records.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.status IS '考勤状态 / Attendance status';


--
-- Name: COLUMN attendance_records.leave_type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.leave_type IS '请假类型 / Leave type';


--
-- Name: COLUMN attendance_records.attendance_days; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.attendance_days IS '出勤天数 / Attendance days';


--
-- Name: COLUMN attendance_records.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.notes IS '备注 / Notes';


--
-- Name: COLUMN attendance_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN attendance_records.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.attendance_records.updated_at IS '更新时间 / Update time';


--
-- Name: attendance_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.attendance_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.attendance_records_id_seq OWNER TO root;

--
-- Name: attendance_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.attendance_records_id_seq OWNED BY public.attendance_records.id;


--
-- Name: detailed_attendance_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.detailed_attendance_records (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    date date NOT NULL,
    morning_status character varying(20) DEFAULT 'absent'::character varying,
    afternoon_status character varying(20) DEFAULT 'absent'::character varying,
    leave_type character varying(20),
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT detailed_attendance_records_afternoon_status_check CHECK (((afternoon_status)::text = ANY ((ARRAY['work'::character varying, 'absent'::character varying])::text[]))),
    CONSTRAINT detailed_attendance_records_morning_status_check CHECK (((morning_status)::text = ANY ((ARRAY['work'::character varying, 'absent'::character varying])::text[])))
);


ALTER TABLE public.detailed_attendance_records OWNER TO root;

--
-- Name: TABLE detailed_attendance_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.detailed_attendance_records IS '详细考勤记录表 / Detailed attendance records table';


--
-- Name: COLUMN detailed_attendance_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.id IS '考勤记录唯一标识符 / Attendance record unique identifier';


--
-- Name: COLUMN detailed_attendance_records.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN detailed_attendance_records.date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.date IS '考勤日期 / Attendance date';


--
-- Name: COLUMN detailed_attendance_records.morning_status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.morning_status IS '上午考勤状态 / Morning attendance status';


--
-- Name: COLUMN detailed_attendance_records.afternoon_status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.afternoon_status IS '下午考勤状态 / Afternoon attendance status';


--
-- Name: COLUMN detailed_attendance_records.leave_type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.leave_type IS '请假类型 / Leave type';


--
-- Name: COLUMN detailed_attendance_records.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.notes IS '备注 / Notes';


--
-- Name: COLUMN detailed_attendance_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN detailed_attendance_records.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.detailed_attendance_records.updated_at IS '更新时间 / Update time';


--
-- Name: attendance_records_view; Type: VIEW; Schema: public; Owner: root
--

CREATE VIEW public.attendance_records_view AS
 SELECT detailed_attendance_records.id,
    detailed_attendance_records.employee_id,
    detailed_attendance_records.date,
        CASE
            WHEN (((detailed_attendance_records.morning_status)::text = 'work'::text) AND ((detailed_attendance_records.afternoon_status)::text = 'work'::text)) THEN 'work'::text
            WHEN (((detailed_attendance_records.morning_status)::text = 'absent'::text) AND ((detailed_attendance_records.afternoon_status)::text = 'absent'::text)) THEN 'absent'::text
            ELSE 'partial'::text
        END AS status,
    detailed_attendance_records.leave_type,
        CASE
            WHEN (((detailed_attendance_records.morning_status)::text = 'work'::text) AND ((detailed_attendance_records.afternoon_status)::text = 'work'::text)) THEN 1.0
            WHEN (((detailed_attendance_records.morning_status)::text = 'work'::text) OR ((detailed_attendance_records.afternoon_status)::text = 'work'::text)) THEN 0.5
            ELSE 0.0
        END AS attendance_days,
    detailed_attendance_records.notes,
    detailed_attendance_records.created_at,
    detailed_attendance_records.updated_at
   FROM public.detailed_attendance_records;


ALTER TABLE public.attendance_records_view OWNER TO root;

--
-- Name: cost_types; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.cost_types (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    category character varying(50) NOT NULL,
    description text,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT cost_types_category_check CHECK (((category)::text = ANY (ARRAY[('salary'::character varying)::text, ('operation'::character varying)::text, ('product'::character varying)::text, ('marketing'::character varying)::text, ('other'::character varying)::text])))
);


ALTER TABLE public.cost_types OWNER TO root;

--
-- Name: cost_types_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.cost_types_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.cost_types_id_seq OWNER TO root;

--
-- Name: cost_types_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.cost_types_id_seq OWNED BY public.cost_types.id;


--
-- Name: customer_orders; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.customer_orders (
    id integer NOT NULL,
    customer_id integer NOT NULL,
    employee_id integer,
    order_number character varying(50) NOT NULL,
    total_amount numeric(12,2) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying,
    order_date date NOT NULL,
    delivery_date date,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT customer_orders_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('confirmed'::character varying)::text, ('delivered'::character varying)::text, ('cancelled'::character varying)::text])))
);


ALTER TABLE public.customer_orders OWNER TO root;

--
-- Name: TABLE customer_orders; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.customer_orders IS '客户订单表 / Customer orders table';


--
-- Name: COLUMN customer_orders.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.id IS '订单唯一标识符 / Order unique identifier';


--
-- Name: COLUMN customer_orders.customer_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.customer_id IS '关联的客户ID / Associated customer ID';


--
-- Name: COLUMN customer_orders.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN customer_orders.order_number; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.order_number IS '订单编号，唯一且必填 / Order number, unique and required';


--
-- Name: COLUMN customer_orders.total_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.total_amount IS '总金额 / Total amount';


--
-- Name: COLUMN customer_orders.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.status IS '状态 / Status';


--
-- Name: COLUMN customer_orders.order_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.order_date IS '订单日期 / Order date';


--
-- Name: COLUMN customer_orders.delivery_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.delivery_date IS '交付日期 / Delivery date';


--
-- Name: COLUMN customer_orders.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.notes IS '备注 / Notes';


--
-- Name: COLUMN customer_orders.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN customer_orders.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customer_orders.updated_at IS '更新时间 / Update time';


--
-- Name: customer_orders_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.customer_orders_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.customer_orders_id_seq OWNER TO root;

--
-- Name: customer_orders_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.customer_orders_id_seq OWNED BY public.customer_orders.id;


--
-- Name: customers; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.customers (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    phone character varying(20),
    email character varying(255),
    company character varying(100),
    address text,
    customer_type character varying(20) DEFAULT 'individual'::character varying,
    status character varying(20) DEFAULT 'active'::character varying,
    credit_limit numeric(12,2) DEFAULT 0,
    balance numeric(12,2) DEFAULT 0,
    created_by integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT customers_customer_type_check CHECK (((customer_type)::text = ANY (ARRAY[('individual'::character varying)::text, ('business'::character varying)::text, ('vip'::character varying)::text]))),
    CONSTRAINT customers_status_check CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text, ('blacklisted'::character varying)::text])))
);


ALTER TABLE public.customers OWNER TO root;

--
-- Name: TABLE customers; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.customers IS '客户表 / Customers table';


--
-- Name: COLUMN customers.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.id IS '客户唯一标识符 / Customer unique identifier';


--
-- Name: COLUMN customers.name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.name IS '客户姓名 / Customer name';


--
-- Name: COLUMN customers.phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.phone IS '电话 / Phone';


--
-- Name: COLUMN customers.email; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.email IS '邮箱 / Email';


--
-- Name: COLUMN customers.company; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.company IS '公司 / Company';


--
-- Name: COLUMN customers.address; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.address IS '地址 / Address';


--
-- Name: COLUMN customers.customer_type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.customer_type IS '客户类型 / Customer type';


--
-- Name: COLUMN customers.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.status IS '状态 / Status';


--
-- Name: COLUMN customers.credit_limit; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.credit_limit IS '信用额度 / Credit limit';


--
-- Name: COLUMN customers.balance; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.balance IS '余额 / Balance';


--
-- Name: COLUMN customers.created_by; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.created_by IS '创建人 / Created by';


--
-- Name: COLUMN customers.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN customers.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.customers.updated_at IS '更新时间 / Update time';


--
-- Name: customers_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.customers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.customers_id_seq OWNER TO root;

--
-- Name: customers_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.customers_id_seq OWNED BY public.customers.id;


--
-- Name: daily_performance_order_details; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.daily_performance_order_details (
    id integer NOT NULL,
    performance_record_id integer NOT NULL,
    parent_phone character varying(20),
    order_id character varying(100),
    wechat_pay_screenshot_url character varying(500),
    wechat_chat_screenshot_url character varying(500),
    wechat_contact_screenshot_url character varying(500),
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    product_id integer,
    approval_status character varying(20) DEFAULT 'pending_approval'::character varying,
    submitted_by integer,
    approved_by integer,
    approved_at timestamp with time zone,
    rejection_reason text,
    employee_id integer,
    work_date date,
    parent_added_teacher_wechat boolean DEFAULT false
);


ALTER TABLE public.daily_performance_order_details OWNER TO root;

--
-- Name: TABLE daily_performance_order_details; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.daily_performance_order_details IS '每日业绩订单明细表，存储每个订单的详细信息';


--
-- Name: COLUMN daily_performance_order_details.performance_record_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_order_details.performance_record_id IS '关联的业绩记录ID';


--
-- Name: COLUMN daily_performance_order_details.parent_phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_order_details.parent_phone IS '家长电话';


--
-- Name: COLUMN daily_performance_order_details.order_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_order_details.order_id IS '订单ID';


--
-- Name: COLUMN daily_performance_order_details.wechat_pay_screenshot_url; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_order_details.wechat_pay_screenshot_url IS '微信支付截图URL';


--
-- Name: COLUMN daily_performance_order_details.wechat_chat_screenshot_url; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_order_details.wechat_chat_screenshot_url IS '微信聊天记录截图URL';


--
-- Name: COLUMN daily_performance_order_details.wechat_contact_screenshot_url; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_order_details.wechat_contact_screenshot_url IS '添加微信首页截图URL';


--
-- Name: daily_performance_order_details_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.daily_performance_order_details_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.daily_performance_order_details_id_seq OWNER TO root;

--
-- Name: daily_performance_order_details_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.daily_performance_order_details_id_seq OWNED BY public.daily_performance_order_details.id;


--
-- Name: daily_performance_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.daily_performance_records (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    work_date date NOT NULL,
    orders_count integer DEFAULT 0,
    sales_amount numeric(12,2) DEFAULT 0,
    commission_amount numeric(12,2) DEFAULT 0,
    product_id integer,
    work_period character varying(10) DEFAULT 'full_day'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT daily_performance_records_work_period_check CHECK (((work_period)::text = ANY ((ARRAY['morning'::character varying, 'afternoon'::character varying, 'full_day'::character varying])::text[])))
);


ALTER TABLE public.daily_performance_records OWNER TO root;

--
-- Name: TABLE daily_performance_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.daily_performance_records IS '每日业绩记录表 / Daily performance records table';


--
-- Name: COLUMN daily_performance_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.id IS '业绩记录唯一标识符 / Performance record unique identifier';


--
-- Name: COLUMN daily_performance_records.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN daily_performance_records.work_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.work_date IS '工作日期 / Work date';


--
-- Name: COLUMN daily_performance_records.orders_count; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.orders_count IS '订单数量 / Number of orders';


--
-- Name: COLUMN daily_performance_records.sales_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.sales_amount IS '销售金额 / Sales amount';


--
-- Name: COLUMN daily_performance_records.commission_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.commission_amount IS '佣金金额 / Commission amount';


--
-- Name: COLUMN daily_performance_records.product_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.product_id IS '产品ID / Product ID';


--
-- Name: COLUMN daily_performance_records.work_period; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.work_period IS '工作时段 / Work period';


--
-- Name: COLUMN daily_performance_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN daily_performance_records.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.daily_performance_records.updated_at IS '更新时间 / Update time';


--
-- Name: daily_performance_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.daily_performance_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.daily_performance_records_id_seq OWNER TO root;

--
-- Name: daily_performance_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.daily_performance_records_id_seq OWNED BY public.daily_performance_records.id;


--
-- Name: detailed_attendance_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.detailed_attendance_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.detailed_attendance_records_id_seq OWNER TO root;

--
-- Name: detailed_attendance_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.detailed_attendance_records_id_seq OWNED BY public.detailed_attendance_records.id;


--
-- Name: employee_level_history; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.employee_level_history (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    from_level character varying(20),
    to_level character varying(20) NOT NULL,
    change_date date NOT NULL,
    reason text,
    approved_by integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    condition_met_date date
);


ALTER TABLE public.employee_level_history OWNER TO root;

--
-- Name: TABLE employee_level_history; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.employee_level_history IS '员工级别变更历史 / Employee level change history';


--
-- Name: COLUMN employee_level_history.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.id IS '历史记录唯一标识符 / History record unique identifier';


--
-- Name: COLUMN employee_level_history.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN employee_level_history.from_level; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.from_level IS '原级别 / From level';


--
-- Name: COLUMN employee_level_history.to_level; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.to_level IS '新级别 / To level';


--
-- Name: COLUMN employee_level_history.change_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.change_date IS '变更日期 / Change date';


--
-- Name: COLUMN employee_level_history.reason; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.reason IS '变更原因 / Reason for change';


--
-- Name: COLUMN employee_level_history.approved_by; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.approved_by IS '批准人 / Approved by';


--
-- Name: COLUMN employee_level_history.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN employee_level_history.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.updated_at IS '更新时间 / Update time';


--
-- Name: COLUMN employee_level_history.condition_met_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employee_level_history.condition_met_date IS '满足晋升条件的日期（用于考核计算，与审批日期change_date区分）';


--
-- Name: employee_level_history_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.employee_level_history_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.employee_level_history_id_seq OWNER TO root;

--
-- Name: employee_level_history_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.employee_level_history_id_seq OWNED BY public.employee_level_history.id;


--
-- Name: employees; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.employees (
    id integer NOT NULL,
    user_id integer,
    employee_code character varying(20) NOT NULL,
    name character varying(100) NOT NULL,
    phone character varying(20),
    email character varying(255),
    department character varying(100),
    "position" character varying(100),
    level character varying(20) DEFAULT 'trainee'::character varying,
    join_date date,
    avatar_url text,
    salary_base numeric(12,2) DEFAULT 0,
    status character varying(20) DEFAULT 'active'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    team_id integer,
    CONSTRAINT employees_level_check CHECK (((level)::text = ANY (ARRAY[('trainee'::character varying)::text, ('c_level'::character varying)::text, ('b_level'::character varying)::text, ('a_level'::character varying)::text]))),
    CONSTRAINT employees_status_check CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text, ('suspended'::character varying)::text])))
);


ALTER TABLE public.employees OWNER TO root;

--
-- Name: TABLE employees; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.employees IS '员工基本信息表 / Employee basic information table';


--
-- Name: COLUMN employees.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.id IS '员工唯一标识符 / Employee unique identifier';


--
-- Name: COLUMN employees.user_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.user_id IS '关联的用户ID / Associated user ID';


--
-- Name: COLUMN employees.employee_code; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.employee_code IS '员工编号，唯一且必填 / Employee code, unique and required';


--
-- Name: COLUMN employees.name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.name IS '员工姓名 / Employee name';


--
-- Name: COLUMN employees.phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.phone IS '员工电话 / Employee phone';


--
-- Name: COLUMN employees.email; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.email IS '员工工作邮箱 / Employee work email';


--
-- Name: COLUMN employees.department; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.department IS '所属部门 / Department';


--
-- Name: COLUMN employees."position"; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees."position" IS '职位 / Position';


--
-- Name: COLUMN employees.level; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.level IS '员工级别 / Employee level';


--
-- Name: COLUMN employees.join_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.join_date IS '入职日期 / Join date';


--
-- Name: COLUMN employees.avatar_url; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.avatar_url IS '头像URL / Avatar URL';


--
-- Name: COLUMN employees.salary_base; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.salary_base IS '基本工资 / Base salary';


--
-- Name: COLUMN employees.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.status IS '员工状态 / Employee status';


--
-- Name: COLUMN employees.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN employees.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.employees.updated_at IS '更新时间 / Update time';


--
-- Name: employees_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.employees_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.employees_id_seq OWNER TO root;

--
-- Name: employees_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.employees_id_seq OWNED BY public.employees.id;


--
-- Name: finance_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.finance_records (
    id integer NOT NULL,
    type character varying(20) NOT NULL,
    category character varying(100) NOT NULL,
    amount numeric(12,2) NOT NULL,
    description text,
    reference_id character varying(100),
    reference_type character varying(50),
    transaction_date date NOT NULL,
    recorded_by integer,
    status character varying(20) DEFAULT 'confirmed'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    order_count integer,
    commission_per_order numeric(15,2),
    customer_name character varying(200),
    product_name character varying(200),
    cost_type character varying(20),
    recurrence_frequency character varying(20),
    recurrence_start_date date,
    recurrence_end_date date,
    total_amount numeric(15,2),
    amortized_daily numeric(15,2),
    total_receivable numeric(15,2),
    received_amount numeric(15,2) DEFAULT 0,
    remaining_amount numeric(15,2),
    cost_type_id integer,
    account_title character varying(255),
    source character varying(50),
    CONSTRAINT finance_records_cost_type_check CHECK (((cost_type)::text = ANY (ARRAY[('onetime'::character varying)::text, ('recurring'::character varying)::text]))),
    CONSTRAINT finance_records_recurrence_frequency_check CHECK (((recurrence_frequency)::text = ANY (ARRAY[('daily'::character varying)::text, ('monthly'::character varying)::text, ('yearly'::character varying)::text]))),
    CONSTRAINT finance_records_status_check CHECK (((status)::text = ANY (ARRAY[('pending'::character varying)::text, ('confirmed'::character varying)::text, ('cancelled'::character varying)::text]))),
    CONSTRAINT finance_records_type_check CHECK (((type)::text = ANY (ARRAY[('income'::character varying)::text, ('expense'::character varying)::text, ('receivable'::character varying)::text])))
);


ALTER TABLE public.finance_records OWNER TO root;

--
-- Name: TABLE finance_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.finance_records IS '财务记录表 / Financial records table';


--
-- Name: COLUMN finance_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.id IS '财务记录唯一标识符 / Financial record unique identifier';


--
-- Name: COLUMN finance_records.type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.type IS '记录类型 / Record type';


--
-- Name: COLUMN finance_records.category; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.category IS '财务类别 / Financial category';


--
-- Name: COLUMN finance_records.amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.amount IS '金额 / Amount';


--
-- Name: COLUMN finance_records.description; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.description IS '描述 / Description';


--
-- Name: COLUMN finance_records.reference_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.reference_id IS '关联ID / Reference ID';


--
-- Name: COLUMN finance_records.reference_type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.reference_type IS '关联类型 / Reference type';


--
-- Name: COLUMN finance_records.transaction_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.transaction_date IS '交易日期 / Transaction date';


--
-- Name: COLUMN finance_records.recorded_by; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.recorded_by IS '记录人 / Recorded by';


--
-- Name: COLUMN finance_records.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.status IS '状态 / Status';


--
-- Name: COLUMN finance_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN finance_records.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.finance_records.updated_at IS '更新时间 / Update time';


--
-- Name: finance_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.finance_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.finance_records_id_seq OWNER TO root;

--
-- Name: finance_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.finance_records_id_seq OWNED BY public.finance_records.id;


--
-- Name: leads; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.leads (
    id integer NOT NULL,
    customer_name character varying(100) NOT NULL,
    phone character varying(20),
    email character varying(255),
    company character varying(100),
    industry character varying(100),
    source character varying(100),
    status character varying(20) DEFAULT 'new'::character varying,
    priority character varying(10) DEFAULT 'medium'::character varying,
    assigned_to integer,
    estimated_value numeric(12,2),
    probability integer,
    next_follow_up date,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT leads_priority_check CHECK (((priority)::text = ANY (ARRAY[('high'::character varying)::text, ('medium'::character varying)::text, ('low'::character varying)::text]))),
    CONSTRAINT leads_probability_check CHECK (((probability >= 0) AND (probability <= 100))),
    CONSTRAINT leads_status_check CHECK (((status)::text = ANY (ARRAY[('new'::character varying)::text, ('contacted'::character varying)::text, ('qualified'::character varying)::text, ('converted'::character varying)::text, ('lost'::character varying)::text])))
);


ALTER TABLE public.leads OWNER TO root;

--
-- Name: TABLE leads; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.leads IS '线索表 / Leads table';


--
-- Name: COLUMN leads.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.id IS '线索唯一标识符 / Lead unique identifier';


--
-- Name: COLUMN leads.customer_name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.customer_name IS '客户姓名 / Customer name';


--
-- Name: COLUMN leads.phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.phone IS '电话 / Phone';


--
-- Name: COLUMN leads.email; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.email IS '邮箱 / Email';


--
-- Name: COLUMN leads.company; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.company IS '公司 / Company';


--
-- Name: COLUMN leads.industry; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.industry IS '行业 / Industry';


--
-- Name: COLUMN leads.source; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.source IS '来源 / Source';


--
-- Name: COLUMN leads.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.status IS '状态 / Status';


--
-- Name: COLUMN leads.priority; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.priority IS '优先级 / Priority';


--
-- Name: COLUMN leads.assigned_to; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.assigned_to IS '分配给 / Assigned to';


--
-- Name: COLUMN leads.estimated_value; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.estimated_value IS '预估价值 / Estimated value';


--
-- Name: COLUMN leads.probability; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.probability IS '成交概率 / Probability of closing';


--
-- Name: COLUMN leads.next_follow_up; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.next_follow_up IS '下次跟进日期 / Next follow-up date';


--
-- Name: COLUMN leads.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.notes IS '备注 / Notes';


--
-- Name: COLUMN leads.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN leads.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.leads.updated_at IS '更新时间 / Update time';


--
-- Name: leads_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.leads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.leads_id_seq OWNER TO root;

--
-- Name: leads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.leads_id_seq OWNED BY public.leads.id;


--
-- Name: monthly_attendance_summary; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.monthly_attendance_summary (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    year integer NOT NULL,
    month integer NOT NULL,
    total_workdays integer DEFAULT 0 NOT NULL,
    actual_work_days integer DEFAULT 0 NOT NULL,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT monthly_attendance_summary_month_check CHECK (((month >= 1) AND (month <= 12)))
);


ALTER TABLE public.monthly_attendance_summary OWNER TO root;

--
-- Name: TABLE monthly_attendance_summary; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.monthly_attendance_summary IS '月度考勤汇总表 - 用于薪资计算和月度统计';


--
-- Name: COLUMN monthly_attendance_summary.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.employee_id IS '员工ID';


--
-- Name: COLUMN monthly_attendance_summary.year; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.year IS '年份';


--
-- Name: COLUMN monthly_attendance_summary.month; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.month IS '月份';


--
-- Name: COLUMN monthly_attendance_summary.total_workdays; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.total_workdays IS '本月总工作日数（从workday_config表读取）';


--
-- Name: COLUMN monthly_attendance_summary.actual_work_days; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.actual_work_days IS '实际出勤天数（考勤状态为work的天数）';


--
-- Name: COLUMN monthly_attendance_summary.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.notes IS '备注信息';


--
-- Name: COLUMN monthly_attendance_summary.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.created_at IS '创建时间';


--
-- Name: COLUMN monthly_attendance_summary.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.monthly_attendance_summary.updated_at IS '更新时间';


--
-- Name: monthly_attendance_summary_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.monthly_attendance_summary_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.monthly_attendance_summary_id_seq OWNER TO root;

--
-- Name: monthly_attendance_summary_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.monthly_attendance_summary_id_seq OWNED BY public.monthly_attendance_summary.id;


--
-- Name: notifications; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.notifications (
    id integer NOT NULL,
    user_id integer NOT NULL,
    title character varying(200) NOT NULL,
    message text NOT NULL,
    type character varying(20) DEFAULT 'info'::character varying,
    category character varying(50) DEFAULT 'general'::character varying,
    is_read boolean DEFAULT false,
    read_at timestamp with time zone,
    metadata jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT notifications_type_check CHECK (((type)::text = ANY (ARRAY[('info'::character varying)::text, ('warning'::character varying)::text, ('error'::character varying)::text, ('success'::character varying)::text])))
);


ALTER TABLE public.notifications OWNER TO root;

--
-- Name: TABLE notifications; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.notifications IS '通知表 / Notifications table';


--
-- Name: COLUMN notifications.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.id IS '通知唯一标识符 / Notification unique identifier';


--
-- Name: COLUMN notifications.user_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.user_id IS '关联的用户ID / Associated user ID';


--
-- Name: COLUMN notifications.title; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.title IS '标题 / Title';


--
-- Name: COLUMN notifications.message; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.message IS '消息内容 / Message content';


--
-- Name: COLUMN notifications.type; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.type IS '类型 / Type';


--
-- Name: COLUMN notifications.category; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.category IS '类别 / Category';


--
-- Name: COLUMN notifications.is_read; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.is_read IS '是否已读 / Is read';


--
-- Name: COLUMN notifications.read_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.read_at IS '阅读时间 / Read time';


--
-- Name: COLUMN notifications.metadata; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.metadata IS '元数据 / Metadata';


--
-- Name: COLUMN notifications.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN notifications.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.notifications.updated_at IS '更新时间 / Update time';


--
-- Name: notifications_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.notifications_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.notifications_id_seq OWNER TO root;

--
-- Name: notifications_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.notifications_id_seq OWNED BY public.notifications.id;


--
-- Name: payment_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.payment_records (
    id integer NOT NULL,
    receivable_id integer NOT NULL,
    amount numeric(15,2) NOT NULL,
    payment_date date NOT NULL,
    payment_method character varying(50) DEFAULT 'bank'::character varying,
    note text,
    recorded_by integer,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.payment_records OWNER TO root;

--
-- Name: payment_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.payment_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.payment_records_id_seq OWNER TO root;

--
-- Name: payment_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.payment_records_id_seq OWNED BY public.payment_records.id;


--
-- Name: performance_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.performance_records (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    period_year integer NOT NULL,
    period_month integer NOT NULL,
    orders_count integer DEFAULT 0,
    sales_amount numeric(12,2) DEFAULT 0,
    new_customers integer DEFAULT 0,
    target_orders integer DEFAULT 0,
    target_amount numeric(12,2) DEFAULT 0,
    commission_amount numeric(12,2) DEFAULT 0,
    bonus_amount numeric(12,2) DEFAULT 0,
    performance_rating character varying(10),
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    product_id integer,
    CONSTRAINT performance_records_performance_rating_check CHECK (((performance_rating)::text = ANY (ARRAY[('A'::character varying)::text, ('B'::character varying)::text, ('C'::character varying)::text, ('D'::character varying)::text]))),
    CONSTRAINT performance_records_period_month_check CHECK (((period_month >= 1) AND (period_month <= 12)))
);


ALTER TABLE public.performance_records OWNER TO root;

--
-- Name: TABLE performance_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.performance_records IS '业绩记录表 / Performance records table';


--
-- Name: COLUMN performance_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.id IS '业绩记录唯一标识符 / Performance record unique identifier';


--
-- Name: COLUMN performance_records.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN performance_records.period_year; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.period_year IS '业绩年份 / Performance year';


--
-- Name: COLUMN performance_records.period_month; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.period_month IS '业绩月份 (1-12) / Performance month (1-12)';


--
-- Name: COLUMN performance_records.orders_count; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.orders_count IS '订单数量 / Number of orders';


--
-- Name: COLUMN performance_records.sales_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.sales_amount IS '销售金额 / Sales amount';


--
-- Name: COLUMN performance_records.new_customers; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.new_customers IS '新客户数量 / Number of new customers';


--
-- Name: COLUMN performance_records.target_orders; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.target_orders IS '目标订单数 / Target orders';


--
-- Name: COLUMN performance_records.target_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.target_amount IS '目标金额 / Target amount';


--
-- Name: COLUMN performance_records.commission_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.commission_amount IS '佣金金额 / Commission amount';


--
-- Name: COLUMN performance_records.bonus_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.bonus_amount IS '奖金金额 / Bonus amount';


--
-- Name: COLUMN performance_records.performance_rating; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.performance_rating IS '绩效评级 / Performance rating';


--
-- Name: COLUMN performance_records.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.notes IS '备注 / Notes';


--
-- Name: COLUMN performance_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN performance_records.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.performance_records.updated_at IS '更新时间 / Update time';


--
-- Name: performance_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.performance_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.performance_records_id_seq OWNER TO root;

--
-- Name: performance_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.performance_records_id_seq OWNED BY public.performance_records.id;


--
-- Name: position_module_permissions; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.position_module_permissions (
    id integer NOT NULL,
    position_name character varying(100) NOT NULL,
    module_key character varying(50) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.position_module_permissions OWNER TO root;

--
-- Name: TABLE position_module_permissions; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.position_module_permissions IS '职位模块权限表 - 存储每个职位可访问的模块 / Position module permissions table';


--
-- Name: COLUMN position_module_permissions.position_name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.position_module_permissions.position_name IS '职位名称，对应 employees.position / Position name';


--
-- Name: COLUMN position_module_permissions.module_key; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.position_module_permissions.module_key IS '模块标识符 / Module key identifier';


--
-- Name: position_module_permissions_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.position_module_permissions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.position_module_permissions_id_seq OWNER TO root;

--
-- Name: position_module_permissions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.position_module_permissions_id_seq OWNED BY public.position_module_permissions.id;


--
-- Name: products; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.products (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    code character varying(20) NOT NULL,
    description text,
    price numeric(10,2) NOT NULL,
    category character varying(50),
    status character varying(20) DEFAULT 'active'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    commission_rules jsonb,
    provider character varying(100),
    CONSTRAINT products_status_check CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text])))
);


ALTER TABLE public.products OWNER TO root;

--
-- Name: TABLE products; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.products IS '产品表 / Products table';


--
-- Name: COLUMN products.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.id IS '产品唯一标识符 / Product unique identifier';


--
-- Name: COLUMN products.name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.name IS '产品名称 / Product name';


--
-- Name: COLUMN products.code; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.code IS '产品编码，唯一且必填 / Product code, unique and required';


--
-- Name: COLUMN products.description; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.description IS '产品描述 / Product description';


--
-- Name: COLUMN products.price; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.price IS '产品价格 / Product price';


--
-- Name: COLUMN products.category; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.category IS '产品分类 / Product category';


--
-- Name: COLUMN products.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.status IS '产品状态 / Product status';


--
-- Name: COLUMN products.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN products.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.updated_at IS '更新时间 / Update time';


--
-- Name: COLUMN products.commission_rules; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.commission_rules IS '提成规则配置（JSON格式）: {"tiers": [{"from": 1, "to": 3, "amount": 10}, ...]}';


--
-- Name: COLUMN products.provider; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.products.provider IS '服务商（如：网易、学而思、高途）';


--
-- Name: products_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.products_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.products_id_seq OWNER TO root;

--
-- Name: products_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.products_id_seq OWNED BY public.products.id;


--
-- Name: salary_records; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.salary_records (
    id integer NOT NULL,
    employee_id integer NOT NULL,
    period_year integer NOT NULL,
    period_month integer NOT NULL,
    base_salary numeric(12,2) DEFAULT 0,
    commission_amount numeric(12,2) DEFAULT 0,
    bonus_amount numeric(12,2) DEFAULT 0,
    overtime_pay numeric(12,2) DEFAULT 0,
    deductions numeric(12,2) DEFAULT 0,
    net_salary numeric(12,2) DEFAULT 0,
    payment_date date,
    payment_status character varying(20) DEFAULT 'pending'::character varying,
    calculation_details jsonb,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT salary_records_payment_status_check CHECK (((payment_status)::text = ANY (ARRAY[('pending'::character varying)::text, ('paid'::character varying)::text, ('cancelled'::character varying)::text]))),
    CONSTRAINT salary_records_period_month_check CHECK (((period_month >= 1) AND (period_month <= 12)))
);


ALTER TABLE public.salary_records OWNER TO root;

--
-- Name: TABLE salary_records; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.salary_records IS '薪资记录表 / Salary records table';


--
-- Name: COLUMN salary_records.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.id IS '薪资记录唯一标识符 / Salary record unique identifier';


--
-- Name: COLUMN salary_records.employee_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.employee_id IS '关联的员工ID / Associated employee ID';


--
-- Name: COLUMN salary_records.period_year; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.period_year IS '薪资年份 / Salary year';


--
-- Name: COLUMN salary_records.period_month; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.period_month IS '薪资月份 / Salary month';


--
-- Name: COLUMN salary_records.base_salary; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.base_salary IS '基本工资 / Base salary';


--
-- Name: COLUMN salary_records.commission_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.commission_amount IS '佣金金额 / Commission amount';


--
-- Name: COLUMN salary_records.bonus_amount; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.bonus_amount IS '奖金金额 / Bonus amount';


--
-- Name: COLUMN salary_records.overtime_pay; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.overtime_pay IS '加班费 / Overtime pay';


--
-- Name: COLUMN salary_records.deductions; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.deductions IS '扣除金额 / Deductions';


--
-- Name: COLUMN salary_records.net_salary; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.net_salary IS '实发工资 / Net salary';


--
-- Name: COLUMN salary_records.payment_date; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.payment_date IS '发放日期 / Payment date';


--
-- Name: COLUMN salary_records.payment_status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.payment_status IS '发放状态 / Payment status';


--
-- Name: COLUMN salary_records.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.notes IS '备注 / Notes';


--
-- Name: COLUMN salary_records.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN salary_records.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.salary_records.updated_at IS '更新时间 / Update time';


--
-- Name: salary_records_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.salary_records_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.salary_records_id_seq OWNER TO root;

--
-- Name: salary_records_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.salary_records_id_seq OWNED BY public.salary_records.id;


--
-- Name: system_settings; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.system_settings (
    key character varying(100) NOT NULL,
    value text,
    description text,
    category character varying(50) NOT NULL,
    is_public boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.system_settings OWNER TO root;

--
-- Name: TABLE system_settings; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.system_settings IS '系统设置表 / System settings table';


--
-- Name: COLUMN system_settings.key; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.key IS '设置键 / Setting key';


--
-- Name: COLUMN system_settings.value; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.value IS '设置值 / Setting value';


--
-- Name: COLUMN system_settings.description; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.description IS '描述 / Description';


--
-- Name: COLUMN system_settings.category; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.category IS '分类 / Category';


--
-- Name: COLUMN system_settings.is_public; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.is_public IS '是否公开 / Is public';


--
-- Name: COLUMN system_settings.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN system_settings.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.system_settings.updated_at IS '更新时间 / Update time';


--
-- Name: teams; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.teams (
    id integer NOT NULL,
    name character varying(100) NOT NULL,
    code character varying(20) NOT NULL,
    description text,
    manager_id integer,
    status character varying(20) DEFAULT 'active'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT teams_status_check CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text])))
);


ALTER TABLE public.teams OWNER TO root;

--
-- Name: TABLE teams; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.teams IS '团队表 / Teams table';


--
-- Name: COLUMN teams.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.id IS '团队唯一标识符 / Team unique identifier';


--
-- Name: COLUMN teams.name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.name IS '团队名称 / Team name';


--
-- Name: COLUMN teams.code; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.code IS '团队编码，唯一且必填 / Team code, unique and required';


--
-- Name: COLUMN teams.description; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.description IS '团队描述 / Team description';


--
-- Name: COLUMN teams.manager_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.manager_id IS '团队经理ID / Team manager ID';


--
-- Name: COLUMN teams.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.status IS '团队状态 / Team status';


--
-- Name: COLUMN teams.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN teams.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.teams.updated_at IS '更新时间 / Update time';


--
-- Name: teams_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.teams_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.teams_id_seq OWNER TO root;

--
-- Name: teams_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.teams_id_seq OWNED BY public.teams.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.users (
    id integer NOT NULL,
    email character varying(255),
    password_hash character varying(255) NOT NULL,
    role character varying(50) DEFAULT 'employee'::character varying,
    status character varying(20) DEFAULT 'active'::character varying,
    last_login timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    phone character varying(20),
    CONSTRAINT users_role_check CHECK (((role)::text = ANY (ARRAY[('admin'::character varying)::text, ('hr'::character varying)::text, ('manager'::character varying)::text, ('workplace_manager'::character varying)::text, ('employee'::character varying)::text]))),
    CONSTRAINT users_status_check CHECK (((status)::text = ANY (ARRAY[('active'::character varying)::text, ('inactive'::character varying)::text, ('suspended'::character varying)::text])))
);


ALTER TABLE public.users OWNER TO root;

--
-- Name: TABLE users; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.users IS '用户表 (用于登录认证) / Users table (for login authentication)';


--
-- Name: COLUMN users.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.id IS '用户唯一标识符 / User unique identifier';


--
-- Name: COLUMN users.email; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.email IS '用户邮箱，唯一且必填 / User email, unique and required';


--
-- Name: COLUMN users.password_hash; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.password_hash IS '密码哈希值 / Password hash value';


--
-- Name: COLUMN users.role; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.role IS '用户角色 / User role';


--
-- Name: COLUMN users.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.status IS '用户状态 / User status';


--
-- Name: COLUMN users.last_login; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.last_login IS '最后登录时间 / Last login time';


--
-- Name: COLUMN users.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN users.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.updated_at IS '更新时间 / Update time';


--
-- Name: COLUMN users.phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.users.phone IS '手机号，用于登录 / Phone number for login';


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.users_id_seq OWNER TO root;

--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: wechat_leads; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.wechat_leads (
    id integer NOT NULL,
    call_record_id character varying(200),
    customer_name character varying(100),
    real_phone character varying(50) NOT NULL,
    encrypted_phone character varying(100),
    call_status character varying(100),
    call_duration integer DEFAULT 0,
    call_time timestamp with time zone,
    audio_url text,
    city character varying(50),
    province character varying(50),
    wechat_added boolean DEFAULT false,
    wechat_add_time timestamp with time zone,
    status character varying(20) DEFAULT 'new'::character varying,
    assigned_to integer,
    assigned_at timestamp with time zone,
    last_follow_up_time timestamp with time zone,
    last_follow_up_content text,
    follow_up_count integer DEFAULT 0,
    is_converted boolean DEFAULT false,
    converted_at timestamp with time zone,
    data_source character varying(50) DEFAULT 'gooki_api'::character varying,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT wechat_leads_status_check CHECK (((status)::text = ANY (ARRAY[('new'::character varying)::text, ('contacted'::character varying)::text, ('qualified'::character varying)::text, ('converted'::character varying)::text, ('lost'::character varying)::text, ('duplicate'::character varying)::text])))
);


ALTER TABLE public.wechat_leads OWNER TO root;

--
-- Name: TABLE wechat_leads; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.wechat_leads IS '微信线索表 MVP 版本 / WeChat leads table MVP version';


--
-- Name: COLUMN wechat_leads.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.id IS '线索唯一标识符 / Lead unique identifier';


--
-- Name: COLUMN wechat_leads.call_record_id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.call_record_id IS '通话记录ID（外部系统）/ Call record ID (external system)';


--
-- Name: COLUMN wechat_leads.customer_name; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.customer_name IS '客户姓名 / Customer name';


--
-- Name: COLUMN wechat_leads.real_phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.real_phone IS '真实手机号 / Real phone number';


--
-- Name: COLUMN wechat_leads.encrypted_phone; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.encrypted_phone IS '加密手机号 / Encrypted phone number';


--
-- Name: COLUMN wechat_leads.call_status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.call_status IS '通话状态 / Call status';


--
-- Name: COLUMN wechat_leads.call_duration; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.call_duration IS '通话时长（秒）/ Call duration (seconds)';


--
-- Name: COLUMN wechat_leads.call_time; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.call_time IS '拨打时间 / Call time';


--
-- Name: COLUMN wechat_leads.audio_url; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.audio_url IS '录音链接 / Audio recording URL';


--
-- Name: COLUMN wechat_leads.city; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.city IS '城市 / City';


--
-- Name: COLUMN wechat_leads.province; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.province IS '省份 / Province';


--
-- Name: COLUMN wechat_leads.wechat_added; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.wechat_added IS '是否已加微信 / WeChat added';


--
-- Name: COLUMN wechat_leads.wechat_add_time; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.wechat_add_time IS '加微信时间 / WeChat add time';


--
-- Name: COLUMN wechat_leads.status; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.status IS '状态 / Status';


--
-- Name: COLUMN wechat_leads.assigned_to; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.assigned_to IS '分配给员工 / Assigned to employee';


--
-- Name: COLUMN wechat_leads.assigned_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.assigned_at IS '分配时间 / Assigned time';


--
-- Name: COLUMN wechat_leads.last_follow_up_time; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.last_follow_up_time IS '最后跟进时间 / Last follow-up time';


--
-- Name: COLUMN wechat_leads.last_follow_up_content; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.last_follow_up_content IS '最后跟进内容 / Last follow-up content';


--
-- Name: COLUMN wechat_leads.follow_up_count; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.follow_up_count IS '跟进次数 / Follow-up count';


--
-- Name: COLUMN wechat_leads.is_converted; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.is_converted IS '是否已转化 / Is converted';


--
-- Name: COLUMN wechat_leads.converted_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.converted_at IS '转化时间 / Converted time';


--
-- Name: COLUMN wechat_leads.data_source; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.data_source IS '数据来源 / Data source';


--
-- Name: COLUMN wechat_leads.notes; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.notes IS '备注 / Notes';


--
-- Name: COLUMN wechat_leads.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN wechat_leads.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.wechat_leads.updated_at IS '更新时间 / Update time';


--
-- Name: wechat_leads_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.wechat_leads_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.wechat_leads_id_seq OWNER TO root;

--
-- Name: wechat_leads_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.wechat_leads_id_seq OWNED BY public.wechat_leads.id;


--
-- Name: workday_config; Type: TABLE; Schema: public; Owner: root
--

CREATE TABLE public.workday_config (
    id integer NOT NULL,
    year integer NOT NULL,
    month integer NOT NULL,
    workdays integer[] NOT NULL,
    total_workdays integer NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT workday_config_month_check CHECK (((month >= 1) AND (month <= 12)))
);


ALTER TABLE public.workday_config OWNER TO root;

--
-- Name: TABLE workday_config; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON TABLE public.workday_config IS '工作日配置表 / Workday configuration table';


--
-- Name: COLUMN workday_config.id; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.id IS '配置记录唯一标识符 / Configuration record unique identifier';


--
-- Name: COLUMN workday_config.year; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.year IS '年份 / Year';


--
-- Name: COLUMN workday_config.month; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.month IS '月份 / Month';


--
-- Name: COLUMN workday_config.workdays; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.workdays IS '工作日列表 / Workdays list';


--
-- Name: COLUMN workday_config.total_workdays; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.total_workdays IS '总工作日数 / Total workdays';


--
-- Name: COLUMN workday_config.created_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.created_at IS '创建时间 / Creation time';


--
-- Name: COLUMN workday_config.updated_at; Type: COMMENT; Schema: public; Owner: root
--

COMMENT ON COLUMN public.workday_config.updated_at IS '更新时间 / Update time';


--
-- Name: workday_config_id_seq; Type: SEQUENCE; Schema: public; Owner: root
--

CREATE SEQUENCE public.workday_config_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.workday_config_id_seq OWNER TO root;

--
-- Name: workday_config_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: root
--

ALTER SEQUENCE public.workday_config_id_seq OWNED BY public.workday_config.id;


--
-- Name: approval_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_records ALTER COLUMN id SET DEFAULT nextval('public.approval_records_id_seq'::regclass);


--
-- Name: approval_requests id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_requests ALTER COLUMN id SET DEFAULT nextval('public.approval_requests_id_seq'::regclass);


--
-- Name: attendance_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.attendance_records ALTER COLUMN id SET DEFAULT nextval('public.attendance_records_id_seq'::regclass);


--
-- Name: cost_types id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.cost_types ALTER COLUMN id SET DEFAULT nextval('public.cost_types_id_seq'::regclass);


--
-- Name: customer_orders id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customer_orders ALTER COLUMN id SET DEFAULT nextval('public.customer_orders_id_seq'::regclass);


--
-- Name: customers id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customers ALTER COLUMN id SET DEFAULT nextval('public.customers_id_seq'::regclass);


--
-- Name: daily_performance_order_details id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details ALTER COLUMN id SET DEFAULT nextval('public.daily_performance_order_details_id_seq'::regclass);


--
-- Name: daily_performance_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_records ALTER COLUMN id SET DEFAULT nextval('public.daily_performance_records_id_seq'::regclass);


--
-- Name: detailed_attendance_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.detailed_attendance_records ALTER COLUMN id SET DEFAULT nextval('public.detailed_attendance_records_id_seq'::regclass);


--
-- Name: employee_level_history id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employee_level_history ALTER COLUMN id SET DEFAULT nextval('public.employee_level_history_id_seq'::regclass);


--
-- Name: employees id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees ALTER COLUMN id SET DEFAULT nextval('public.employees_id_seq'::regclass);


--
-- Name: finance_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.finance_records ALTER COLUMN id SET DEFAULT nextval('public.finance_records_id_seq'::regclass);


--
-- Name: leads id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.leads ALTER COLUMN id SET DEFAULT nextval('public.leads_id_seq'::regclass);


--
-- Name: monthly_attendance_summary id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.monthly_attendance_summary ALTER COLUMN id SET DEFAULT nextval('public.monthly_attendance_summary_id_seq'::regclass);


--
-- Name: notifications id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.notifications ALTER COLUMN id SET DEFAULT nextval('public.notifications_id_seq'::regclass);


--
-- Name: payment_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.payment_records ALTER COLUMN id SET DEFAULT nextval('public.payment_records_id_seq'::regclass);


--
-- Name: performance_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.performance_records ALTER COLUMN id SET DEFAULT nextval('public.performance_records_id_seq'::regclass);


--
-- Name: position_module_permissions id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.position_module_permissions ALTER COLUMN id SET DEFAULT nextval('public.position_module_permissions_id_seq'::regclass);


--
-- Name: products id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.products ALTER COLUMN id SET DEFAULT nextval('public.products_id_seq'::regclass);


--
-- Name: salary_records id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.salary_records ALTER COLUMN id SET DEFAULT nextval('public.salary_records_id_seq'::regclass);


--
-- Name: teams id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.teams ALTER COLUMN id SET DEFAULT nextval('public.teams_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: wechat_leads id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.wechat_leads ALTER COLUMN id SET DEFAULT nextval('public.wechat_leads_id_seq'::regclass);


--
-- Name: workday_config id; Type: DEFAULT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.workday_config ALTER COLUMN id SET DEFAULT nextval('public.workday_config_id_seq'::regclass);


--
-- Name: approval_records approval_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_records
    ADD CONSTRAINT approval_records_pkey PRIMARY KEY (id);


--
-- Name: approval_requests approval_requests_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_pkey PRIMARY KEY (id);


--
-- Name: attendance_records attendance_records_employee_id_date_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.attendance_records
    ADD CONSTRAINT attendance_records_employee_id_date_key UNIQUE (employee_id, date);


--
-- Name: attendance_records attendance_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.attendance_records
    ADD CONSTRAINT attendance_records_pkey PRIMARY KEY (id);


--
-- Name: cost_types cost_types_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.cost_types
    ADD CONSTRAINT cost_types_pkey PRIMARY KEY (id);


--
-- Name: customer_orders customer_orders_order_number_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customer_orders
    ADD CONSTRAINT customer_orders_order_number_key UNIQUE (order_number);


--
-- Name: customer_orders customer_orders_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customer_orders
    ADD CONSTRAINT customer_orders_pkey PRIMARY KEY (id);


--
-- Name: customers customers_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);


--
-- Name: daily_performance_order_details daily_performance_order_details_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details
    ADD CONSTRAINT daily_performance_order_details_pkey PRIMARY KEY (id);


--
-- Name: daily_performance_records daily_performance_records_employee_date_product_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_records
    ADD CONSTRAINT daily_performance_records_employee_date_product_key UNIQUE (employee_id, work_date, product_id);


--
-- Name: daily_performance_records daily_performance_records_employee_product_date_unique; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_records
    ADD CONSTRAINT daily_performance_records_employee_product_date_unique UNIQUE (employee_id, product_id, work_date);


--
-- Name: daily_performance_records daily_performance_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_records
    ADD CONSTRAINT daily_performance_records_pkey PRIMARY KEY (id);


--
-- Name: detailed_attendance_records detailed_attendance_records_employee_id_date_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.detailed_attendance_records
    ADD CONSTRAINT detailed_attendance_records_employee_id_date_key UNIQUE (employee_id, date);


--
-- Name: detailed_attendance_records detailed_attendance_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.detailed_attendance_records
    ADD CONSTRAINT detailed_attendance_records_pkey PRIMARY KEY (id);


--
-- Name: employee_level_history employee_level_history_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employee_level_history
    ADD CONSTRAINT employee_level_history_pkey PRIMARY KEY (id);


--
-- Name: employees employees_employee_code_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_employee_code_key UNIQUE (employee_code);


--
-- Name: employees employees_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_pkey PRIMARY KEY (id);


--
-- Name: finance_records finance_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.finance_records
    ADD CONSTRAINT finance_records_pkey PRIMARY KEY (id);


--
-- Name: leads leads_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.leads
    ADD CONSTRAINT leads_pkey PRIMARY KEY (id);


--
-- Name: monthly_attendance_summary monthly_attendance_summary_employee_id_year_month_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.monthly_attendance_summary
    ADD CONSTRAINT monthly_attendance_summary_employee_id_year_month_key UNIQUE (employee_id, year, month);


--
-- Name: monthly_attendance_summary monthly_attendance_summary_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.monthly_attendance_summary
    ADD CONSTRAINT monthly_attendance_summary_pkey PRIMARY KEY (id);


--
-- Name: notifications notifications_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_pkey PRIMARY KEY (id);


--
-- Name: payment_records payment_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.payment_records
    ADD CONSTRAINT payment_records_pkey PRIMARY KEY (id);


--
-- Name: performance_records performance_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.performance_records
    ADD CONSTRAINT performance_records_pkey PRIMARY KEY (id);


--
-- Name: performance_records performance_records_unique_employee_period_product; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.performance_records
    ADD CONSTRAINT performance_records_unique_employee_period_product UNIQUE (employee_id, period_year, period_month, product_id);


--
-- Name: position_module_permissions position_module_permissions_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.position_module_permissions
    ADD CONSTRAINT position_module_permissions_pkey PRIMARY KEY (id);


--
-- Name: products products_code_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_code_key UNIQUE (code);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);


--
-- Name: salary_records salary_records_employee_id_period_year_period_month_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.salary_records
    ADD CONSTRAINT salary_records_employee_id_period_year_period_month_key UNIQUE (employee_id, period_year, period_month);


--
-- Name: salary_records salary_records_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.salary_records
    ADD CONSTRAINT salary_records_pkey PRIMARY KEY (id);


--
-- Name: system_settings system_settings_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.system_settings
    ADD CONSTRAINT system_settings_pkey PRIMARY KEY (key);


--
-- Name: teams teams_code_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_code_key UNIQUE (code);


--
-- Name: teams teams_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_pkey PRIMARY KEY (id);


--
-- Name: daily_performance_order_details unique_parent_product; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details
    ADD CONSTRAINT unique_parent_product UNIQUE (parent_phone, product_id);


--
-- Name: position_module_permissions unique_position_module; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.position_module_permissions
    ADD CONSTRAINT unique_position_module UNIQUE (position_name, module_key);


--
-- Name: users users_phone_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_phone_key UNIQUE (phone);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: wechat_leads wechat_leads_call_record_id_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.wechat_leads
    ADD CONSTRAINT wechat_leads_call_record_id_key UNIQUE (call_record_id);


--
-- Name: wechat_leads wechat_leads_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.wechat_leads
    ADD CONSTRAINT wechat_leads_pkey PRIMARY KEY (id);


--
-- Name: workday_config workday_config_pkey; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.workday_config
    ADD CONSTRAINT workday_config_pkey PRIMARY KEY (id);


--
-- Name: workday_config workday_config_year_month_key; Type: CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.workday_config
    ADD CONSTRAINT workday_config_year_month_key UNIQUE (year, month);


--
-- Name: idx_approval_records_request_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_approval_records_request_id ON public.approval_records USING btree (request_id);


--
-- Name: idx_approval_requests_status; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_approval_requests_status ON public.approval_requests USING btree (status);


--
-- Name: idx_cost_types_category; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_cost_types_category ON public.cost_types USING btree (category);


--
-- Name: idx_customer_orders_customer_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_customer_orders_customer_id ON public.customer_orders USING btree (customer_id);


--
-- Name: idx_customers_type; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_customers_type ON public.customers USING btree (customer_type);


--
-- Name: idx_employee_level_history_approved_by; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employee_level_history_approved_by ON public.employee_level_history USING btree (approved_by);


--
-- Name: idx_employee_level_history_change_date; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employee_level_history_change_date ON public.employee_level_history USING btree (change_date);


--
-- Name: idx_employee_level_history_employee_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employee_level_history_employee_id ON public.employee_level_history USING btree (employee_id);


--
-- Name: idx_employees_department; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employees_department ON public.employees USING btree (department);


--
-- Name: idx_employees_employee_code; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employees_employee_code ON public.employees USING btree (employee_code);


--
-- Name: idx_employees_level; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employees_level ON public.employees USING btree (level);


--
-- Name: idx_employees_team_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employees_team_id ON public.employees USING btree (team_id);


--
-- Name: idx_employees_user_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_employees_user_id ON public.employees USING btree (user_id);


--
-- Name: idx_finance_cost_type; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_finance_cost_type ON public.finance_records USING btree (cost_type_id);


--
-- Name: idx_finance_date; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_finance_date ON public.finance_records USING btree (transaction_date);


--
-- Name: idx_finance_reference; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_finance_reference ON public.finance_records USING btree (reference_type, reference_id);


--
-- Name: idx_finance_type; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_finance_type ON public.finance_records USING btree (type);


--
-- Name: idx_leads_assigned_to; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_leads_assigned_to ON public.leads USING btree (assigned_to);


--
-- Name: idx_leads_status; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_leads_status ON public.leads USING btree (status);


--
-- Name: idx_monthly_attendance_employee; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_monthly_attendance_employee ON public.monthly_attendance_summary USING btree (employee_id);


--
-- Name: idx_monthly_attendance_employee_year_month; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_monthly_attendance_employee_year_month ON public.monthly_attendance_summary USING btree (employee_id, year, month);


--
-- Name: idx_monthly_attendance_year_month; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_monthly_attendance_year_month ON public.monthly_attendance_summary USING btree (year, month);


--
-- Name: idx_notifications_read; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_notifications_read ON public.notifications USING btree (is_read);


--
-- Name: idx_notifications_user_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_notifications_user_id ON public.notifications USING btree (user_id);


--
-- Name: idx_order_details_approval; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_order_details_approval ON public.daily_performance_order_details USING btree (approval_status);


--
-- Name: idx_order_details_employee; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_order_details_employee ON public.daily_performance_order_details USING btree (performance_record_id);


--
-- Name: idx_payment_records_date; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_payment_records_date ON public.payment_records USING btree (payment_date);


--
-- Name: idx_payment_records_receivable; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_payment_records_receivable ON public.payment_records USING btree (receivable_id);


--
-- Name: idx_performance_employee_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_performance_employee_id ON public.performance_records USING btree (employee_id);


--
-- Name: idx_performance_order_details_order_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_performance_order_details_order_id ON public.daily_performance_order_details USING btree (order_id);


--
-- Name: idx_performance_order_details_record_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_performance_order_details_record_id ON public.daily_performance_order_details USING btree (performance_record_id);


--
-- Name: idx_performance_period; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_performance_period ON public.performance_records USING btree (period_year, period_month);


--
-- Name: idx_performance_product_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_performance_product_id ON public.performance_records USING btree (product_id);


--
-- Name: idx_position_module_permissions_module; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_position_module_permissions_module ON public.position_module_permissions USING btree (module_key);


--
-- Name: idx_position_module_permissions_position; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_position_module_permissions_position ON public.position_module_permissions USING btree (position_name);


--
-- Name: idx_products_category; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_products_category ON public.products USING btree (category);


--
-- Name: idx_products_code; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_products_code ON public.products USING btree (code);


--
-- Name: idx_products_commission_rules; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_products_commission_rules ON public.products USING gin (commission_rules);


--
-- Name: idx_products_status; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_products_status ON public.products USING btree (status);


--
-- Name: idx_salary_employee_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_salary_employee_id ON public.salary_records USING btree (employee_id);


--
-- Name: idx_salary_period; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_salary_period ON public.salary_records USING btree (period_year, period_month);


--
-- Name: idx_teams_code; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_teams_code ON public.teams USING btree (code);


--
-- Name: idx_teams_manager_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_teams_manager_id ON public.teams USING btree (manager_id);


--
-- Name: idx_teams_status; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_teams_status ON public.teams USING btree (status);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: idx_users_phone; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_users_phone ON public.users USING btree (phone);


--
-- Name: idx_users_role; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_users_role ON public.users USING btree (role);


--
-- Name: idx_wechat_leads_assigned_to; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_assigned_to ON public.wechat_leads USING btree (assigned_to);


--
-- Name: idx_wechat_leads_call_record_id; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_call_record_id ON public.wechat_leads USING btree (call_record_id);


--
-- Name: idx_wechat_leads_call_time; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_call_time ON public.wechat_leads USING btree (call_time);


--
-- Name: idx_wechat_leads_city; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_city ON public.wechat_leads USING btree (city);


--
-- Name: idx_wechat_leads_created_at; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_created_at ON public.wechat_leads USING btree (created_at);


--
-- Name: idx_wechat_leads_province; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_province ON public.wechat_leads USING btree (province);


--
-- Name: idx_wechat_leads_real_phone; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_real_phone ON public.wechat_leads USING btree (real_phone);


--
-- Name: idx_wechat_leads_status; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_status ON public.wechat_leads USING btree (status);


--
-- Name: idx_wechat_leads_wechat_added; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_wechat_leads_wechat_added ON public.wechat_leads USING btree (wechat_added);


--
-- Name: idx_workday_config_year_month; Type: INDEX; Schema: public; Owner: root
--

CREATE INDEX idx_workday_config_year_month ON public.workday_config USING btree (year, month);


--
-- Name: workday_config trigger_sync_monthly_attendance_workdays; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER trigger_sync_monthly_attendance_workdays AFTER INSERT OR UPDATE OF total_workdays ON public.workday_config FOR EACH ROW EXECUTE FUNCTION public.sync_monthly_attendance_workdays();


--
-- Name: monthly_attendance_summary trigger_update_monthly_attendance_summary_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER trigger_update_monthly_attendance_summary_updated_at BEFORE UPDATE ON public.monthly_attendance_summary FOR EACH ROW EXECUTE FUNCTION public.update_monthly_attendance_summary_updated_at();


--
-- Name: approval_requests update_approval_requests_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_approval_requests_updated_at BEFORE UPDATE ON public.approval_requests FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: cost_types update_cost_types_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_cost_types_updated_at BEFORE UPDATE ON public.cost_types FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: customer_orders update_customer_orders_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_customer_orders_updated_at BEFORE UPDATE ON public.customer_orders FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: customers update_customers_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_customers_updated_at BEFORE UPDATE ON public.customers FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: employees update_employees_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_employees_updated_at BEFORE UPDATE ON public.employees FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: finance_records update_finance_records_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_finance_records_updated_at BEFORE UPDATE ON public.finance_records FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: leads update_leads_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_leads_updated_at BEFORE UPDATE ON public.leads FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: notifications update_notifications_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_notifications_updated_at BEFORE UPDATE ON public.notifications FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: payment_records update_payment_records_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_payment_records_updated_at BEFORE UPDATE ON public.payment_records FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: performance_records update_performance_records_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_performance_records_updated_at BEFORE UPDATE ON public.performance_records FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: position_module_permissions update_position_module_permissions_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_position_module_permissions_updated_at BEFORE UPDATE ON public.position_module_permissions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: products update_products_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON public.products FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: salary_records update_salary_records_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_salary_records_updated_at BEFORE UPDATE ON public.salary_records FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: teams update_teams_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_teams_updated_at BEFORE UPDATE ON public.teams FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: users update_users_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: wechat_leads update_wechat_leads_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_wechat_leads_updated_at BEFORE UPDATE ON public.wechat_leads FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: workday_config update_workday_config_updated_at; Type: TRIGGER; Schema: public; Owner: root
--

CREATE TRIGGER update_workday_config_updated_at BEFORE UPDATE ON public.workday_config FOR EACH ROW EXECUTE FUNCTION public.update_updated_at_column();


--
-- Name: approval_records approval_records_approver_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_records
    ADD CONSTRAINT approval_records_approver_id_fkey FOREIGN KEY (approver_id) REFERENCES public.employees(id);


--
-- Name: approval_records approval_records_next_approver_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_records
    ADD CONSTRAINT approval_records_next_approver_id_fkey FOREIGN KEY (next_approver_id) REFERENCES public.employees(id);


--
-- Name: approval_records approval_records_request_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_records
    ADD CONSTRAINT approval_records_request_id_fkey FOREIGN KEY (request_id) REFERENCES public.approval_requests(id) ON DELETE CASCADE;


--
-- Name: approval_requests approval_requests_current_approver_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_current_approver_fkey FOREIGN KEY (current_approver) REFERENCES public.employees(id);


--
-- Name: approval_requests approval_requests_requested_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.approval_requests
    ADD CONSTRAINT approval_requests_requested_by_fkey FOREIGN KEY (requested_by) REFERENCES public.employees(id);


--
-- Name: attendance_records attendance_records_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.attendance_records
    ADD CONSTRAINT attendance_records_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: customer_orders customer_orders_customer_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customer_orders
    ADD CONSTRAINT customer_orders_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES public.customers(id) ON DELETE CASCADE;


--
-- Name: customer_orders customer_orders_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customer_orders
    ADD CONSTRAINT customer_orders_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id);


--
-- Name: customers customers_created_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.customers
    ADD CONSTRAINT customers_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.employees(id);


--
-- Name: daily_performance_order_details daily_performance_order_details_approved_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details
    ADD CONSTRAINT daily_performance_order_details_approved_by_fkey FOREIGN KEY (approved_by) REFERENCES public.employees(id);


--
-- Name: daily_performance_order_details daily_performance_order_details_performance_record_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details
    ADD CONSTRAINT daily_performance_order_details_performance_record_id_fkey FOREIGN KEY (performance_record_id) REFERENCES public.daily_performance_records(id) ON DELETE CASCADE;


--
-- Name: daily_performance_order_details daily_performance_order_details_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details
    ADD CONSTRAINT daily_performance_order_details_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id);


--
-- Name: daily_performance_order_details daily_performance_order_details_submitted_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_order_details
    ADD CONSTRAINT daily_performance_order_details_submitted_by_fkey FOREIGN KEY (submitted_by) REFERENCES public.employees(id);


--
-- Name: daily_performance_records daily_performance_records_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_records
    ADD CONSTRAINT daily_performance_records_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: daily_performance_records daily_performance_records_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.daily_performance_records
    ADD CONSTRAINT daily_performance_records_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id);


--
-- Name: detailed_attendance_records detailed_attendance_records_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.detailed_attendance_records
    ADD CONSTRAINT detailed_attendance_records_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: employee_level_history employee_level_history_approved_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employee_level_history
    ADD CONSTRAINT employee_level_history_approved_by_fkey FOREIGN KEY (approved_by) REFERENCES public.employees(id);


--
-- Name: employee_level_history employee_level_history_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employee_level_history
    ADD CONSTRAINT employee_level_history_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: employees employees_team_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_team_id_fkey FOREIGN KEY (team_id) REFERENCES public.teams(id) ON DELETE SET NULL;


--
-- Name: employees employees_team_id_fkey1; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_team_id_fkey1 FOREIGN KEY (team_id) REFERENCES public.teams(id) ON DELETE SET NULL;


--
-- Name: employees employees_team_id_fkey2; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_team_id_fkey2 FOREIGN KEY (team_id) REFERENCES public.teams(id) ON DELETE SET NULL;


--
-- Name: employees employees_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.employees
    ADD CONSTRAINT employees_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: finance_records finance_records_cost_type_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.finance_records
    ADD CONSTRAINT finance_records_cost_type_id_fkey FOREIGN KEY (cost_type_id) REFERENCES public.cost_types(id);


--
-- Name: finance_records finance_records_recorded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.finance_records
    ADD CONSTRAINT finance_records_recorded_by_fkey FOREIGN KEY (recorded_by) REFERENCES public.employees(id);


--
-- Name: leads leads_assigned_to_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.leads
    ADD CONSTRAINT leads_assigned_to_fkey FOREIGN KEY (assigned_to) REFERENCES public.employees(id);


--
-- Name: monthly_attendance_summary monthly_attendance_summary_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.monthly_attendance_summary
    ADD CONSTRAINT monthly_attendance_summary_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: notifications notifications_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.notifications
    ADD CONSTRAINT notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: payment_records payment_records_receivable_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.payment_records
    ADD CONSTRAINT payment_records_receivable_id_fkey FOREIGN KEY (receivable_id) REFERENCES public.finance_records(id) ON DELETE CASCADE;


--
-- Name: payment_records payment_records_recorded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.payment_records
    ADD CONSTRAINT payment_records_recorded_by_fkey FOREIGN KEY (recorded_by) REFERENCES public.employees(id);


--
-- Name: performance_records performance_records_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.performance_records
    ADD CONSTRAINT performance_records_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: performance_records performance_records_product_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.performance_records
    ADD CONSTRAINT performance_records_product_id_fkey FOREIGN KEY (product_id) REFERENCES public.products(id);


--
-- Name: salary_records salary_records_employee_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.salary_records
    ADD CONSTRAINT salary_records_employee_id_fkey FOREIGN KEY (employee_id) REFERENCES public.employees(id) ON DELETE CASCADE;


--
-- Name: teams teams_manager_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.teams
    ADD CONSTRAINT teams_manager_id_fkey FOREIGN KEY (manager_id) REFERENCES public.employees(id);


--
-- Name: wechat_leads wechat_leads_assigned_to_fkey; Type: FK CONSTRAINT; Schema: public; Owner: root
--

ALTER TABLE ONLY public.wechat_leads
    ADD CONSTRAINT wechat_leads_assigned_to_fkey FOREIGN KEY (assigned_to) REFERENCES public.employees(id);


--
-- PostgreSQL database dump complete
--

\unrestrict ncknEsajz3RMpZze6RZlOQfSiw2xuPASc7zKv2hFTQjA5GwOozruvRxdSG40we7

